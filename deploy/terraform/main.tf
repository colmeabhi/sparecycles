# SpareCycles on AWS EC2: 3 coordinators in 3 availability zones, volunteers
# on spot (or on-demand) instances, and one Prometheus instance.
#
# Instances download the Linux binaries from an S3 bucket that this file
# creates, and run them as systemd services that restart on failure.

locals {
  zones = slice(data.aws_availability_zones.available.names, 0, 3)

  # Coordinators get fixed private IPs, so each one can list the others in SC_PEERS.
  coordinator_ids = [1, 2, 3]
  coordinator_ips = { for i, id in local.coordinator_ids : id => cidrhost(aws_subnet.public[i].cidr_block, 10) }

  coordinator_addrs = [for id in local.coordinator_ids : "${local.coordinator_ips[id]}:7400"]
  # "name=token" gives "token", as in SC_VOLUNTEER_TOKENS.
  volunteer_token = replace(var.volunteer_tokens[0], "/^[^=]*=/", "")

  binaries = toset(["coordinator", "volunteer", "client"])

  instance_profile = var.instance_profile_name != "" ? var.instance_profile_name : aws_iam_instance_profile.node[0].name
  key_name         = var.ssh_key_name != "" ? var.ssh_key_name : null
}

data "aws_availability_zones" "available" {
  state = "available"
}

# Latest Amazon Linux 2023. It has the AWS CLI, which user_data uses to fetch the binaries.
data "aws_ssm_parameter" "al2023" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

# ---------------------------------------------------------------------------
# Network: one VPC with a public subnet in each of 3 zones
# ---------------------------------------------------------------------------

resource "aws_vpc" "main" {
  cidr_block           = "10.40.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = var.name }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = var.name }
}

resource "aws_subnet" "public" {
  count                   = 3
  vpc_id                  = aws_vpc.main.id
  availability_zone       = local.zones[count.index]
  cidr_block              = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index + 1)
  map_public_ip_on_launch = true

  tags = { Name = "${var.name}-${local.zones[count.index]}" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = { Name = var.name }
}

resource "aws_route_table_association" "public" {
  count          = 3
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# ---------------------------------------------------------------------------
# Security groups
# ---------------------------------------------------------------------------

resource "aws_security_group" "coordinator" {
  name        = "${var.name}-coordinator"
  description = "SpareCycles coordinators"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group" "volunteer" {
  name        = "${var.name}-volunteer"
  description = "SpareCycles volunteers"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group" "monitor" {
  name        = "${var.name}-monitor"
  description = "SpareCycles Prometheus"
  vpc_id      = aws_vpc.main.id
}

# Coordinators: gRPC and metrics inside the VPC; gRPC, metrics and SSH from our laptops.
resource "aws_vpc_security_group_ingress_rule" "coordinator_vpc" {
  for_each          = toset(["7400", "9400"])
  security_group_id = aws_security_group.coordinator.id
  cidr_ipv4         = aws_vpc.main.cidr_block
  ip_protocol       = "tcp"
  from_port         = tonumber(each.value)
  to_port           = tonumber(each.value)
  description       = "Port ${each.value} inside the VPC"
}

resource "aws_vpc_security_group_ingress_rule" "coordinator_laptops" {
  for_each          = { for pair in setproduct(var.allowed_cidrs, ["22", "7400", "9400"]) : "${pair[0]}-${pair[1]}" => pair }
  security_group_id = aws_security_group.coordinator.id
  cidr_ipv4         = each.value[0]
  ip_protocol       = "tcp"
  from_port         = tonumber(each.value[1])
  to_port           = tonumber(each.value[1])
  description       = "Port ${each.value[1]} from a laptop"
}

# Volunteers: gossip from the coordinators and the other volunteers; SSH from our laptops.
resource "aws_vpc_security_group_ingress_rule" "volunteer_gossip" {
  security_group_id = aws_security_group.volunteer.id
  cidr_ipv4         = aws_vpc.main.cidr_block
  ip_protocol       = "tcp"
  from_port         = 7500
  to_port           = 7500
  description       = "SWIM gossip inside the VPC"
}

resource "aws_vpc_security_group_ingress_rule" "volunteer_ssh" {
  for_each          = toset(var.allowed_cidrs)
  security_group_id = aws_security_group.volunteer.id
  cidr_ipv4         = each.value
  ip_protocol       = "tcp"
  from_port         = 22
  to_port           = 22
  description       = "SSH from a laptop"
}

resource "aws_vpc_security_group_ingress_rule" "monitor_laptops" {
  for_each          = { for pair in setproduct(var.allowed_cidrs, ["22", "9090"]) : "${pair[0]}-${pair[1]}" => pair }
  security_group_id = aws_security_group.monitor.id
  cidr_ipv4         = each.value[0]
  ip_protocol       = "tcp"
  from_port         = tonumber(each.value[1])
  to_port           = tonumber(each.value[1])
  description       = "Port ${each.value[1]} from a laptop"
}

resource "aws_vpc_security_group_egress_rule" "all" {
  for_each          = { coordinator = aws_security_group.coordinator.id, volunteer = aws_security_group.volunteer.id, monitor = aws_security_group.monitor.id }
  security_group_id = each.value
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "All outbound (S3, package downloads, other nodes)"
}

# ---------------------------------------------------------------------------
# Binaries: an S3 bucket that only our instances can read
# ---------------------------------------------------------------------------

resource "aws_s3_bucket" "binaries" {
  bucket_prefix = "${var.name}-bin-"
  force_destroy = true # terraform destroy removes the binaries too
}

resource "aws_s3_bucket_public_access_block" "binaries" {
  bucket                  = aws_s3_bucket.binaries.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_object" "binary" {
  for_each = local.binaries
  bucket   = aws_s3_bucket.binaries.id
  key      = each.value
  source   = "${var.binaries_dir}/${each.value}"
  etag     = filemd5("${var.binaries_dir}/${each.value}")
}

# Instance role: read the binaries bucket. Skipped when instance_profile_name is set (AWS Academy).
resource "aws_iam_role" "node" {
  count = var.instance_profile_name == "" ? 1 : 0
  name  = "${var.name}-node"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

resource "aws_iam_role_policy" "read_binaries" {
  count = var.instance_profile_name == "" ? 1 : 0
  name  = "read-binaries"
  role  = aws_iam_role.node[0].id
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "s3:GetObject", Resource = "${aws_s3_bucket.binaries.arn}/*" }]
  })
}

resource "aws_iam_instance_profile" "node" {
  count = var.instance_profile_name == "" ? 1 : 0
  name  = "${var.name}-node"
  role  = aws_iam_role.node[0].name
}

# ---------------------------------------------------------------------------
# Instances
# ---------------------------------------------------------------------------

resource "aws_instance" "coordinator" {
  for_each               = local.coordinator_ips
  ami                    = data.aws_ssm_parameter.al2023.value
  instance_type          = var.coordinator_instance_type
  subnet_id              = aws_subnet.public[each.key - 1].id
  private_ip             = each.value
  vpc_security_group_ids = [aws_security_group.coordinator.id]
  iam_instance_profile   = local.instance_profile
  key_name               = local.key_name

  user_data_replace_on_change = true
  user_data = templatefile("${path.module}/templates/node.sh.tftpl", {
    bucket = aws_s3_bucket.binaries.id
    binary = "coordinator"
    etag   = aws_s3_object.binary["coordinator"].etag
    env = {
      SC_NODE_ID          = each.key
      SC_LISTEN_ADDR      = ":7400"
      SC_ADVERTISE_ADDR   = "${each.value}:7400"
      SC_PEERS            = join(",", [for id, ip in local.coordinator_ips : "${id}=${ip}:7400" if id != each.key])
      SC_METRICS_ADDR     = ":9400"
      SC_VOLUNTEER_TOKENS = join(",", var.volunteer_tokens)
      SC_SIGNING_KEY      = var.signing_key
    }
  })

  tags = { Name = "${var.name}-coordinator-${each.key}", Role = "coordinator" }
}

resource "aws_instance" "volunteer" {
  count                  = var.volunteer_count
  ami                    = data.aws_ssm_parameter.al2023.value
  instance_type          = var.volunteer_instance_type
  subnet_id              = aws_subnet.public[count.index % 3].id
  vpc_security_group_ids = [aws_security_group.volunteer.id]
  iam_instance_profile   = local.instance_profile
  key_name               = local.key_name

  dynamic "instance_market_options" {
    for_each = var.volunteer_spot ? [1] : []
    content {
      market_type = "spot"
      spot_options {
        instance_interruption_behavior = "terminate"
      }
    }
  }

  user_data_replace_on_change = true
  user_data = templatefile("${path.module}/templates/node.sh.tftpl", {
    bucket = aws_s3_bucket.binaries.id
    binary = "volunteer"
    etag   = aws_s3_object.binary["volunteer"].etag
    env = {
      SC_COORDINATORS       = join(",", local.coordinator_addrs)
      SC_VOLUNTEER_TOKEN    = local.volunteer_token
      SC_VOLUNTEER_NAME     = "aws-${count.index + 1}"
      SC_SIGNING_PUBLIC_KEY = var.signing_public_key
      SC_GOSSIP_ADDR        = ":7500"
      SC_GOSSIP_HOST        = "hostname"
    }
  })

  # Coordinators first, so that volunteers register at once.
  depends_on = [aws_instance.coordinator]

  tags = { Name = "${var.name}-volunteer-${count.index + 1}", Role = "volunteer" }
}

resource "aws_instance" "monitor" {
  ami                    = data.aws_ssm_parameter.al2023.value
  instance_type          = var.monitor_instance_type
  subnet_id              = aws_subnet.public[0].id
  vpc_security_group_ids = [aws_security_group.monitor.id]
  key_name               = local.key_name

  user_data_replace_on_change = true
  user_data = templatefile("${path.module}/templates/prometheus.sh.tftpl", {
    targets = [for ip in values(local.coordinator_ips) : "${ip}:9400"]
  })

  tags = { Name = "${var.name}-prometheus", Role = "monitor" }
}
