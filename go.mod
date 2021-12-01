module github.com/in4it/ecs-deploy

go 1.15

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43

require (
	github.com/appleboy/gin-jwt/v2 v2.7.0
	github.com/aws/aws-sdk-go v1.41.11
	github.com/aws/aws-sdk-go-v2/service/ecs v1.13.0
	github.com/crewjam/saml v0.4.6-0.20210521115923-29c6295245bd
	github.com/ghodss/yaml v1.0.0
	github.com/gin-contrib/location v0.0.2
	github.com/gin-gonic/gin v1.7.4
	github.com/golang-jwt/jwt/v4 v4.1.0
	github.com/google/go-cmp v0.5.6
	github.com/gorilla/context v1.1.1
	github.com/gorilla/sessions v1.1.3
	github.com/guregu/dynamo v1.2.1
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8
	github.com/robbiet480/go.sns v0.0.0-20181124163742-ca087b49e1da
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/gin-swagger v1.1.0
	github.com/swaggo/swag v1.5.1
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/tools v0.0.0-20190621195816-6e04913cbbac // indirect
)
