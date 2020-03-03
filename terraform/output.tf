output "alb-dns-name" {
  value = aws_alb.alb.dns_name
}

output "alb-zone-id" {
  value = aws_alb.alb.zone_id
}

output "alb-sg" {
  value = aws_security_group.alb.id
}

output "paramstore-kms-arn" {
  value = data.aws_kms_key.ssm.arn
}

output "sg-cluster" {
  value = aws_security_group.cluster.id
}

output "cluster-ec2-role-arn" {
  value = aws_iam_role.cluster-ec2-role.arn
}

output "cluster-ec2-role-name" {
  value = aws_iam_role.cluster-ec2-role.name
}
