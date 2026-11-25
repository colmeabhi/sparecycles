variable "region" {
  description = "AWS region. AWS Academy labs allow only us-east-1."
  type        = string
  default     = "us-east-1"
}

variable "name" {
  description = "Prefix for every resource name and the Project tag."
  type        = string
  default     = "sparecycles"
}

variable "allowed_cidrs" {
  description = "Addresses (CIDR) of our laptops. They may reach gRPC (7400), metrics (9400), Prometheus (9090) and SSH (22). Example: [\"203.0.113.7/32\"]."
  type        = list(string)

  validation {
    condition     = length(var.allowed_cidrs) > 0 && alltrue([for c in var.allowed_cidrs : can(cidrhost(c, 0))])
    error_message = "Set allowed_cidrs to at least one valid CIDR, for example your IP address with /32."
  }
}

variable "binaries_dir" {
  description = "Folder with the Linux binaries from make deploy-build."
  type        = string
  default     = "../../dist"
}

variable "ssh_key_name" {
  description = "Name of an existing EC2 key pair for SSH. Empty means no SSH key (AWS Academy: vockey)."
  type        = string
  default     = ""
}

variable "instance_profile_name" {
  description = "Existing instance profile that can read the binaries bucket. Empty means Terraform creates one. AWS Academy cannot create IAM roles: use LabInstanceProfile."
  type        = string
  default     = ""
}

variable "coordinator_instance_type" {
  description = "Instance type of the 3 coordinators."
  type        = string
  default     = "t3.small"
}

variable "volunteer_count" {
  description = "Number of volunteer instances. Laptops can join as more volunteers."
  type        = number
  default     = 3

  validation {
    condition     = var.volunteer_count >= 0
    error_message = "volunteer_count must be 0 or more."
  }
}

variable "volunteer_instance_type" {
  description = "Instance type of the volunteers."
  type        = string
  default     = "t3.small"
}

variable "volunteer_spot" {
  description = "Run volunteers on spot instances (cheaper; AWS may stop them, which the leases handle). false means on-demand."
  type        = bool
  default     = true
}

variable "monitor_instance_type" {
  description = "Instance type of the Prometheus instance."
  type        = string
  default     = "t3.micro"
}

variable "volunteer_tokens" {
  description = "SC_VOLUNTEER_TOKENS for the coordinators: name=token pairs. The first one is also used by the EC2 volunteers. Keep it in terraform.tfvars, which git ignores."
  type        = list(string)
  sensitive   = true

  validation {
    condition     = length(var.volunteer_tokens) > 0
    error_message = "Set at least one volunteer token, for example [\"aws=some-long-secret\"]."
  }
}

variable "signing_key" {
  description = "SC_SIGNING_KEY (base64 Ed25519 seed) from make keys. Empty means tasks are not signed."
  type        = string
  default     = ""
  sensitive   = true
}

variable "signing_public_key" {
  description = "SC_SIGNING_PUBLIC_KEY that matches signing_key. Empty means volunteers do not check signatures."
  type        = string
  default     = ""
}
