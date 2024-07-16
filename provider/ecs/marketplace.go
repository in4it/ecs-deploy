package ecs

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/marketplacemetering"
)

// Marketplace struct
type Marketplace struct {
}

func (a *Marketplace) RegisterMarketplace() error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(os.Getenv("AWS_REGION"))})
	if err != nil {
		return fmt.Errorf("couldn't initialize S3: %s", err)
	}

	productCode := os.Getenv("PROD_CODE")

	// Create a MarketplaceMetering client from just a session.
	svc := marketplacemetering.New(sess)

	_, err = svc.RegisterUsage(&marketplacemetering.RegisterUsageInput{
		ProductCode:      aws.String(productCode),
		PublicKeyVersion: aws.Int64(1),
	})

	if err != nil {
		if strings.Contains(err.Error(), "CustomerNotEntitledException") {
			fmt.Printf("Exited: no valid subscription found. Subscribe for ecs-deploy on the AWS Marketplace.")
			os.Exit(1)
		}
		return fmt.Errorf("RegisterUsage error: %s", err)
	}

	return nil
}
