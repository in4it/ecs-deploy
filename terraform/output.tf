output "alb-dns-name" {
  value = "${aws_alb.alb.dns_name}"
}

output "alb-zone-id" {
  value = "${aws_alb.alb.zone_id}"
}
