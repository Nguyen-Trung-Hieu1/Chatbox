data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}

resource "aws_iam_role" "ssm" {
  name = "${var.name}-ec2-ssm"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole" //nhan va sd role
    }]
  })
}
//gan quyen ssm vao role
resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.ssm.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "ssm" {
  name = "${var.name}-ec2-ssm"
  role = aws_iam_role.ssm.name
}

resource "aws_security_group" "web" {
  name        = "${var.name}-web"
  description = "Public web access for Chatbox; administration uses Session Manager."
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "HTTP redirects to HTTPS"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS for Chatbox and Grafana"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name}-web" }
}

resource "aws_instance" "chatbox" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  subnet_id                   = data.aws_subnets.default.ids[0]
  vpc_security_group_ids      = [aws_security_group.web.id]
  iam_instance_profile        = aws_iam_instance_profile.ssm.name
  associate_public_ip_address = true
  monitoring                  = false
  # Ubuntu's SSM Agent availability varies by AMI release. Install and start
  # it explicitly on first boot so administration never needs an SSH rule.
  user_data = <<-USERDATA
    #!/usr/bin/env bash
    set -euxo pipefail
    apt-get update
    apt-get install -y snapd
    snap install amazon-ssm-agent --classic || true
    systemctl enable snap.amazon-ssm-agent.amazon-ssm-agent.service || true
    systemctl restart snap.amazon-ssm-agent.amazon-ssm-agent.service || true
  USERDATA
  user_data_replace_on_change = true

  lifecycle {
    ignore_changes = [ami, user_data]
  }

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 2
  }

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.root_volume_size_gib
    iops                  = 3000
    throughput            = 125
    encrypted             = true
    delete_on_termination = true
  }

  tags = { Name = "${var.name}" }
}

resource "aws_eip" "chatbox" {
  domain = "vpc"
  tags   = { Name = "${var.name}" }
}

resource "aws_eip_association" "chatbox" {
  allocation_id = aws_eip.chatbox.id
  instance_id   = aws_instance.chatbox.id
}
