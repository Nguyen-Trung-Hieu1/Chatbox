variable "account_id" {
  description = "New AWS account ID. This guard prevents an apply against the old account."
  type        = string
  default     = "421344955349"
}

variable "aws_region" {
  description = "AWS region for the single-node cluster."
  type        = string
  default     = "ap-southeast-2"
}

variable "instance_type" {
  description = "EC2 size. c7i-flex.large matches the former Chatbox server."
  type        = string
  default     = "c7i-flex.large"
}

variable "name" {
  description = "Resource name prefix."
  type        = string
  default     = "chatbox"
}

variable "root_volume_size_gib" {
  description = "Root EBS size for K3s images, MongoDB, monitoring and 9router data."
  type        = number
  default     = 40
}
