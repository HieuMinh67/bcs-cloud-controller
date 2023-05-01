package networking

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
	"testing"
)

type VPC struct {
	TestObject *testing.T
	AwsRegion  string
}

func (vpc VPC) CreateDefaultVPCIfNotExists() {
	ec2Client := terratest_aws.NewEc2Client(vpc.TestObject, vpc.AwsRegion)

	// https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.CreateVpc
	//println("Creating Default VPC if not exists")
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#DescribeVpcsInput
	vpcsInput := &ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("is-default"),
				Values: []*string{
					aws.String("true"),
				},
			},
		},
	}

	vpcsRes, err := ec2Client.DescribeVpcs(vpcsInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}

	if len(vpcsRes.Vpcs) == 0 {
		vpcInput := &ec2.CreateDefaultVpcInput{}

		_, err := ec2Client.CreateDefaultVpc(vpcInput)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return
		}
	}
}
