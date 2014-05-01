r53ddns
=======

It is an Amazon Route 53 Dynamic DNS updater program.

Install
-------

```bash
go get -u github.com/oov/r53ddns
```

Usage
-----

1. Create "Hosted Zone" and insert A record on Route 53. (d53ddns does not support new A record creation, update only)
2. Create IAM user and configure user policies.
3. Execute d53ddns.

```bash
env AWS_ACCESS_KEY_ID=YOUR-ACCESS-KEY AWS_SECRET_ACCESS_KEY=YOUR-SECRET-KEY r53ddns --zone=YOUR-ZONE-ID --domain=ddns.example.org.
```

### User policy example

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "route53:ListResourceRecordSets",
                "route53:ChangeResourceRecordSets"
            ],
            "Resource": "arn:aws:route53:::hostedzone/YOUR-ZONE-ID"
        },
        {
            "Effect": "Allow",
            "Action": [
                "route53:GetChange"
            ],
            "Resource": "arn:aws:route53:::change/*"
        }
    ]
}
```
