output "instance_id" {
  value = aws_instance.chatbox.id
}

output "elastic_ip" {
  value = aws_eip.chatbox.public_ip
}

output "ssh_is_not_open" {
  value = true
}
