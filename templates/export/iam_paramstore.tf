resource "aws_iam_role_policy" "ecs-${SERVICE}-paramstore" {
    name = "paramstore-${SERVICE}"
    role = "${aws_iam_role.ecs-${SERVICE}.id}"
    policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Action": [
          "ssm:GetParameterHistory",
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath"
        ],
        "Resource": [
          "arn:aws:ssm:${AWS_REGION}:${ACCOUNT_ID}:parameter/${PARAMSTORE_PREFIX}-${AWS_ACCOUNT_ENV}/${NAMESPACE}/*"
        ],
        "Effect": "Allow"
      },
      {
        "Action": [
          "kms:Decrypt"
        ],
        "Resource": [
          "${PARAMSTORE_KMS_ARN}"
        ],
        "Effect": "Allow"
      }
    ]
}
EOF
}
