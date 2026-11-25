output "coordinator_public_ips" {
  description = "Public IP of each coordinator, by node ID. Node 3 leads first."
  value       = { for id, instance in aws_instance.coordinator : id => instance.public_ip }
}

output "sc_coordinators" {
  description = "Put this line in the .env of a laptop client or volunteer."
  value       = "SC_COORDINATORS=${join(",", [for id in sort(keys(aws_instance.coordinator)) : "${aws_instance.coordinator[id].public_ip}:7400"])}"
}

output "prometheus_url" {
  description = "Prometheus web page. It scrapes all 3 coordinators."
  value       = "http://${aws_instance.monitor.public_ip}:9090"
}

output "volunteer_public_ips" {
  description = "Public IP of each EC2 volunteer."
  value       = aws_instance.volunteer[*].public_ip
}

output "ssh_commands" {
  description = "SSH into each coordinator (needs ssh_key_name). Logs: journalctl -u sparecycles -f"
  value       = { for id, instance in aws_instance.coordinator : id => "ssh -i <your key file> ec2-user@${instance.public_ip}" }
}

output "kill_leader_command" {
  description = "Failure test: stop the first leader (node 3). Node 2 takes over."
  value       = "aws ec2 stop-instances --region ${var.region} --instance-ids ${aws_instance.coordinator["3"].id}"
}
