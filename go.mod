module github.com/in4it/ecs-deploy

go 1.12

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43

require (
	github.com/appleboy/gin-jwt v2.5.0+incompatible
	github.com/appleboy/gin-jwt/v2 v2.6.2
	github.com/aws/aws-sdk-go v1.29.34
	github.com/beevik/etree v1.1.0 // indirect
	github.com/crewjam/saml v0.0.0-20170522121329-6b5dd2d26974
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/gin-contrib/location v0.0.0-20190528141421-4d994432eb13
	github.com/gin-gonic/gin v1.4.0
	github.com/google/go-cmp v0.3.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/sessions v1.1.3
	github.com/guregu/dynamo v1.2.1
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8
	github.com/robbiet480/go.sns v0.0.0-20181124163742-ca087b49e1da
	github.com/russellhaering/goxmldsig v0.0.0-20180430223755-7acd5e4a6ef7 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/swaggo/gin-swagger v1.1.0
	github.com/swaggo/swag v1.5.1
	golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529
	gopkg.in/dgrijalva/jwt-go.v3 v3.2.0 // indirect
	honnef.co/go/tools v0.0.1-2019.2.3 // indirect
)
