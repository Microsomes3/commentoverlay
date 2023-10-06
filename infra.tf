terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.16"
    }
  }

  required_version = ">= 1.2.0"
}

provider "aws" {
  region  = "us-east-1"
}

variable "ubuntu-ami" {
  type    = string
  default = "ami-053b0d53c279acc90"
}

resource "aws_key_pair" "masterkey2" {
  key_name   = "von2"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "aws_instance" "app_server" {
  ami           = var.ubuntu-ami
  instance_type = "c3.4xlarge"
  key_name      = aws_key_pair.masterkey2.key_name  

  tags = {
    Name = "MasterNode"
  }
  root_block_device {
    volume_size = 50
  }

    vpc_security_group_ids = [aws_security_group.allow_ssh.id]  # Replace with the actual security group ID

}



# This resource triggers the execution of the Ansible playbook
resource "null_resource" "run_ansible3" {
  depends_on = [aws_instance.app_server]

  provisioner "local-exec" {
    command = "sleep 60 && ansible-playbook -i ${aws_instance.app_server.public_ip}, setup.yml --ssh-extra-args='-o StrictHostKeyChecking=no'"
  }
}



resource "aws_security_group" "allow_ssh" {
  name        = "allow_tls2"
  description = "Allow ssh inbound traffic"

  tags = {
    Name = "allow_ssh"
  }
}

resource "aws_security_group_rule" "allow_ssh" {
  type        = "ingress"
  from_port   = 22
  to_port     = 22
  protocol    = "tcp"
  cidr_blocks = ["0.0.0.0/0"]  # Allowing SSH access from any IP. Replace with your desired CIDR block.
  
  security_group_id = aws_security_group.allow_ssh.id
}


resource "aws_security_group_rule" "allow_http" {
  type              = "ingress"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.allow_ssh.id
}


resource "aws_security_group_rule" "allow_outbound" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"  # Indicates all protocols
  cidr_blocks       = ["0.0.0.0/0"]
  
  security_group_id = aws_security_group.allow_ssh.id
}


output "ip" {

    value = aws_instance.app_server.public_ip  
}