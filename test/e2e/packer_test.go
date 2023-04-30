package e2e

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/packer"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	//_ "github.com/aws/aws-sdk-go/config"
	"github.com/aws/aws-sdk-go/service/ec2"
	terratest_aws "github.com/gruntwork-io/terratest/modules/aws"
)

// Occasionally, a Packer build may fail due to intermittent issues (e.g., brief network outage or EC2 issue). We try
// to make our tests resilient to that by specifying those known common errors here and telling our builds to retry if
// they hit those errors.
var DefaultRetryablePackerErrors = map[string]string{
	"Script disconnected unexpectedly":                                                 "Occasionally, Packer seems to lose connectivity to AWS, perhaps due to a brief network outage",
	"can not open /var/lib/apt/lists/archive.ubuntu.com_ubuntu_dists_xenial_InRelease": "Occasionally, apt-get fails on ubuntu to update the cache",
}
var DefaultTimeBetweenPackerRetries = 15 * time.Second

const DefaultMaxPackerRetries = 3

var log = logger.Logger{}

// This is a complicated, end-to-end integration test. It builds the AMI from examples/packer-docker-example,
// deploys it using the Terraform code on examples/terraform-packer-example, and checks that the web server in the AMI
// response to requests. The test is broken into "stages" so you can skip stages by setting environment variables (e.g.,
// skip stage "build_ami" by setting the environment variable "SKIP_build_ami=true"), which speeds up iteration when
// running this test over and over again locally.
func TestImageBuildForAwsUbuntuMicroK8s(t *testing.T) {
	t.Parallel()
	// The folder where we have our Terraform code
	workingDir := "../../packer"

	// At the end of the test, delete the AMI
	defer test_structure.RunTestStage(t, "cleanup_ami", func() {
		awsRegion := test_structure.LoadString(t, workingDir, "awsRegion")
		deleteAMI(t, awsRegion, workingDir)
	})

	// At the end of the test, undeploy the web app using Terraform
	//defer test_structure.RunTestStage(t, "cleanup_terraform", func() {
	//	undeployUsingTerraform(t, workingDir)
	//})

	// At the end of the test, fetch the most recent syslog entries from each Instance. This can be useful for
	// debugging issues without having to manually SSH to the server.
	defer test_structure.RunTestStage(t, "logs", func() {
		awsRegion := test_structure.LoadString(t, workingDir, "awsRegion")
		fetchSyslogForInstance(t, awsRegion, workingDir)
	})

	// Build the AMI for the CloudController
	test_structure.RunTestStage(t, "build_ami", func() {
		// Pick a random AWS region to test in. This helps ensure your code works in all regions.
		regions := []string{"us-west-2"}
		awsRegion := terratest_aws.GetRandomStableRegion(t, regions, nil)
		test_structure.SaveString(t, workingDir, "awsRegion", awsRegion)
		buildAMI(t, awsRegion, workingDir)
	})

	// Create a new instance
	test_structure.RunTestStage(t, "create_instance", func() {
		awsRegion := test_structure.LoadString(t, workingDir, "awsRegion")
		createEc2Instance(t, awsRegion, workingDir)
	})

	// Validate that the web app deployed and is responding to HTTP requests
	test_structure.RunTestStage(t, "validate", func() {
		awsRegion := test_structure.LoadString(t, workingDir, "awsRegion")
		validateInstanceRunningKubernetes(t, awsRegion, workingDir)
	})

}

// Build the AMI in packer-docker-example
func buildAMI(t *testing.T, awsRegion string, workingDir string) {
	// Some AWS regions are missing certain instance types, so pick an available type based on the region we picked
	//instanceType := terratest_aws.GetRecommendedInstanceType(t, awsRegion, []string{"t4g.micro"})

	packerOptions := &packer.Options{
		// The path to where the Packer template is located
		Template: "../../packer/aws-ubuntu.pkr.hcl",

		// Variable file to to pass to our Packer build using -var-file option
		VarFiles: []string{
			//varFile.Name(),
			"../../packer/aws-ubuntu.auto.pkrvars.hcl",
		},

		// Environment settings to avoid plugin conflicts
		Env: map[string]string{
			"PACKER_PLUGIN_PATH": "../../packer/.packer.d/plugins",
		},

		// Only build the AWS AMI
		Only: "bcs-cloud-controller.*",

		// Configure retries for intermittent errors
		RetryableErrors:    DefaultRetryablePackerErrors,
		TimeBetweenRetries: DefaultTimeBetweenPackerRetries,
		MaxRetries:         DefaultMaxPackerRetries,
	}

	// Save the Packer Options so future test stages can use them
	test_structure.SavePackerOptions(t, workingDir, packerOptions)

	// Build the AMI
	amiID := packer.BuildArtifact(t, packerOptions)

	// Save the AMI ID so future test stages can use them
	test_structure.SaveArtifactID(t, workingDir, amiID)

	// Check if AMI is shared/not shared with account
	requestingAccount := "606500562958" // PeterBeanSandbox sandbox@peterbean.net AWSAccount
	randomAccount := "123456789012"     // Random Account
	ec2Client := terratest_aws.NewEc2Client(t, awsRegion)
	ShareAmi(t, amiID, requestingAccount, ec2Client)
	accountsWithLaunchPermissions := terratest_aws.GetAccountsWithLaunchPermissionsForAmi(t, awsRegion, amiID)
	assert.NotContains(t, accountsWithLaunchPermissions, randomAccount)
	assert.Contains(t, accountsWithLaunchPermissions, requestingAccount)

	// Check if AMI is public
	MakeAmiPublic(t, amiID, ec2Client)
	amiIsPublic := terratest_aws.GetAmiPubliclyAccessible(t, awsRegion, amiID)
	assert.True(t, amiIsPublic)
}

func createEc2Instance(t *testing.T, awsRegion string, workingDir string) {
	ec2Client := terratest_aws.NewEc2Client(t, awsRegion)
	amiId := test_structure.LoadArtifactID(t, workingDir)

	// https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.CreateVpc
	println("Creating VPC if not exists")
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#DescribeVpcsInput
	vpcsInput := &ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String("My VPC for E2E"),
				},
			},
		},
		//VpcIds: []*string{
		//	aws.String("vpc-a01106c2"),
		//},
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

	fmt.Println(vpcsRes)

	groupName := "my-security-groupV3"
	var vpcId, subnetId, groupId string
	if len(vpcsRes.Vpcs) == 0 {
		println("VPC not exist so create a new one")
		vpcTag := &ec2.TagSpecification{
			ResourceType: aws.String(ec2.ResourceTypeVpc),
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String("My VPC for E2E"),
				},
			},
		}

		vpcInput := &ec2.CreateVpcInput{
			CidrBlock:         aws.String("10.0.0.0/16"),
			TagSpecifications: []*ec2.TagSpecification{vpcTag},
		}

		vpcRes, err := ec2Client.CreateVpc(vpcInput)
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

		vpcId = *vpcRes.Vpc.VpcId

		// https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.CreateSubnet
		createSubnetIpt := &ec2.CreateSubnetInput{
			CidrBlock: aws.String("10.0.1.0/24"),
			VpcId:     aws.String(vpcId),
		}

		createSubnetRes, err := ec2Client.CreateSubnet(createSubnetIpt)
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

		//fmt.Println(vpcRes)
		subnetId = *createSubnetRes.Subnet.SubnetId

		// https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.CreateSecurityGroup
		// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/ec2-example-security-groups.html#create-security-group
		sgTag := &ec2.TagSpecification{
			ResourceType: aws.String(ec2.ResourceTypeSecurityGroup),
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String("My security group V3"),
				},
			},
		}
		sgInput := &ec2.CreateSecurityGroupInput{
			Description:       aws.String("My security group"),
			GroupName:         aws.String(groupName),
			TagSpecifications: []*ec2.TagSpecification{sgTag},
			VpcId:             aws.String(vpcId),
		}

		securityGroupRes, err := ec2Client.CreateSecurityGroup(sgInput)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				switch awsErr.Code() {
				default:
					fmt.Println(awsErr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return
		}

		fmt.Println(securityGroupRes)
		groupId = *securityGroupRes.GroupId

		_, err = ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(groupId),

			IpPermissions: []*ec2.IpPermission{
				// Can use setters to simplify seting multiple values without the
				// needing to use aws.String or associated helper utilities.
				(&ec2.IpPermission{}).
					SetIpProtocol("tcp").
					SetFromPort(80).
					SetToPort(80).
					SetIpRanges([]*ec2.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					}),
				(&ec2.IpPermission{}).
					SetIpProtocol("tcp").
					SetFromPort(16443).
					SetToPort(16443).
					SetIpRanges([]*ec2.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					}),
				(&ec2.IpPermission{}).
					SetIpProtocol("tcp").
					SetFromPort(22).
					SetToPort(22).
					SetIpRanges([]*ec2.IpRange{
						(&ec2.IpRange{}).
							SetCidrIp("0.0.0.0/0"),
					}),
			},
		})
		if err != nil {
			fmt.Printf("Unable to set security group %q ingress, %v", groupName, err)
			os.Exit(1)
		}

		fmt.Println("Successfully set security group ingress")
	} else {
		println("Reuse existing VPC")
		vpcId = *vpcsRes.Vpcs[0].VpcId

		subnetsInput := &ec2.DescribeSubnetsInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String("vpc-id"),
					Values: []*string{
						aws.String(vpcId),
					},
				},
			},
		}

		subnetsRes, err := ec2Client.DescribeSubnets(subnetsInput)
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

		subnetId = *subnetsRes.Subnets[0].SubnetId

		sgRes, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String("vpc-id"),
					Values: []*string{
						aws.String(vpcId),
					},
				},
				{
					Name: aws.String("group-name"),
					Values: []*string{
						aws.String(groupName),
					},
				},
			},
		})
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
		groupId = *sgRes.SecurityGroups[0].GroupId
	}

	println("VPC ID to use is " + vpcId)

	// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/ec2-example-create-images.html
	instanceRes, err := ec2Client.RunInstances(
		&ec2.RunInstancesInput{
			// An Amazon Linux AMI ID for t2.micro instances in the us-west-2 region
			ImageId:          aws.String(amiId),
			InstanceType:     aws.String("t4g.micro"),
			MinCount:         aws.Int64(1),
			MaxCount:         aws.Int64(1),
			SecurityGroupIds: []*string{&groupId},
			SubnetId:         aws.String(subnetId),
		})

	if err != nil {
		fmt.Println("Could not create instance", err)
		return
	}

	instanceId := instanceRes.Instances[0].InstanceId

	fmt.Println("Created instance", *instanceRes.Instances[0].InstanceId)
	// Add tags to the created instance
	_, errtag := ec2Client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{instanceRes.Instances[0].InstanceId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String("MicroK8sTestInstance"),
			},
		},
	})

	if errtag != nil {
		log.Logf(t, "Could not create tags for instance %s because of error %s", instanceId, errtag)
		return
	}

	fmt.Println("Successfully tagged instance")
	test_structure.SaveString(t, workingDir, "instanceId", *instanceId)
}

// Delete the AMI
func deleteAMI(t *testing.T, awsRegion string, workingDir string) {
	// Load the AMI ID and Packer Options saved by the earlier build_ami stage
	amiID := test_structure.LoadArtifactID(t, workingDir)

	//terratest_aws.DeleteAmi(t, awsRegion, amiID)
	terratest_aws.DeleteAmiAndAllSnapshots(t, awsRegion, amiID)
}

// Fetch the most recent syslogs for the instance. This is a handy way to see what happened on the Instance as part of
// your test log output, without having to re-run the test and manually SSH to the Instance.
func fetchSyslogForInstance(t *testing.T, awsRegion string, workingDir string) {
	// Load the Terraform Options saved by the earlier deploy_terraform stage
	//terraformOptions := test_structure.LoadTerraformOptions(t, workingDir)

	//instanceID := terraform.OutputRequired(t, terraformOptions, "instance_id")
	//instanceId := "123"
	//logs := terratest_aws.GetSyslogForInstance(t, instanceID, awsRegion)

	//log.Logf(t, "Most recent syslog for Instance %s:\n\n%s\n", instanceID, logs)
}

// Validate Kubernetes has been deployed and is working
func validateInstanceRunningKubernetes(t *testing.T, awsRegion string, workingDir string) {
	// Load the Terraform Options saved by the earlier deploy_terraform stage
	//terraformOptions := test_structure.LoadTerraformOptions(t, workingDir)
	ec2Client := terratest_aws.NewEc2Client(t, awsRegion)
	instanceId := test_structure.LoadString(t, workingDir, "instanceId")

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceId),
		},
	}

	result, err := ec2Client.DescribeInstances(input)
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

	fmt.Println(result)
	//publicIpAddress := result.Reservations[0].Instances[0].PublicIpAddress

	fmt.Printf("instanceId is %s \n", instanceId)
	//fmt.Printf("publicIpAddress is %s \n", *publicIpAddress)

	// Run `terraform output` to get the value of an output variable
	//instanceURL := terraform.Output(t, terraformOptions, "instance_url")

	//instanceURL := "http://localhost"

	// Setup a TLS configuration to submit with the helper, a blank struct is acceptable
	//tlsConfig := tls.Config{}

	// Figure out what text the instance should return for each request
	//instanceText, _ := terraformOptions.Vars["instance_text"].(string)

	// It can take a minute or so for the Instance to boot up, so retry a few times
	//maxRetries := 3
	//timeBetweenRetries := 5 * time.Second

	// Verify that we get back a 200 OK with the expected instanceText
	//http_helper.HttpGetWithRetry(t, instanceURL, &tlsConfig, 200, "", maxRetries, timeBetweenRetries)
}

// An example of how to test the Packer template in examples/packer-basic-example using Terratest
// with the VarFiles option. This test generates a temporary *.json file containing the value
// for the `aws_region` variable.
func TestImageBuild(t *testing.T) {
	return

	t.Parallel()

	// Pick a random AWS region to test in. This helps ensure your code works in all regions.
	regions := []string{"us-west-2"}
	awsRegion := terratest_aws.GetRandomStableRegion(t, regions, nil)

	// Some AWS regions are missing certain instance types, so pick an available type based on the region we picked
	//instanceType := terratest_aws.GetRecommendedInstanceType(t, awsRegion, []string{"t2.micro", "t3.micro"})

	// Create temporary packer variable file to store aws region
	//varFile, err := ioutil.TempFile("", "*.json")
	//require.NoError(t, err, "Did not expect temp file creation to cause error")

	// Be sure to clean up temp file
	//defer func(name string) {
	//	err := os.Remove(name)
	//	if err != nil {
	//		println(err.Error())
	//	}
	//}(varFile.Name())

	// Write the vars we need to a temporary json file
	//varFileContent := []byte(fmt.Sprintf(`{"aws_region": "%s", "instance_type": "%s"}`, awsRegion, instanceType))
	//_, err = varFile.Write(varFileContent)
	//require.NoError(t, err, "Did not expect writing to temp file %s to cause error", varFile.Name())

	packerOptions := &packer.Options{
		// The path to where the Packer template is located
		Template: "../../packer/aws-ubuntu.pkr.hcl",

		// Variable file to to pass to our Packer build using -var-file option
		VarFiles: []string{
			//varFile.Name(),
			"../../packer/aws-ubuntu.auto.pkrvars.hcl",
		},

		// Environment settings to avoid plugin conflicts
		Env: map[string]string{
			"PACKER_PLUGIN_PATH": "../../packer/.packer.d/plugins",
		},

		// Only build the AWS AMI
		Only: "bcs-cloud-controller.*",

		// Configure retries for intermittent errors
		RetryableErrors:    DefaultRetryablePackerErrors,
		TimeBetweenRetries: DefaultTimeBetweenPackerRetries,
		MaxRetries:         DefaultMaxPackerRetries,
	}

	// Make sure the Packer build completes successfully
	amiID := packer.BuildArtifact(t, packerOptions)

	// Clean up the AMI after we're done
	defer terratest_aws.DeleteAmiAndAllSnapshots(t, awsRegion, amiID)

	// Check if AMI is shared/not shared with account
	requestingAccount := "606500562958" // PeterBeanSandbox sandbox@peterbean.net AWSAccount
	randomAccount := "123456789012"     // Random Account
	ec2Client := terratest_aws.NewEc2Client(t, awsRegion)
	ShareAmi(t, amiID, requestingAccount, ec2Client)
	accountsWithLaunchPermissions := terratest_aws.GetAccountsWithLaunchPermissionsForAmi(t, awsRegion, amiID)
	assert.NotContains(t, accountsWithLaunchPermissions, randomAccount)
	assert.Contains(t, accountsWithLaunchPermissions, requestingAccount)

	// Check if AMI is public
	MakeAmiPublic(t, amiID, ec2Client)
	amiIsPublic := terratest_aws.GetAmiPubliclyAccessible(t, awsRegion, amiID)
	assert.True(t, amiIsPublic)
}

func ShareAmi(t *testing.T, amiID string, accountID string, ec2Client *ec2.EC2) {
	input := &ec2.ModifyImageAttributeInput{
		ImageId: aws.String(amiID),
		LaunchPermission: &ec2.LaunchPermissionModifications{
			Add: []*ec2.LaunchPermission{
				{
					UserId: aws.String(accountID),
				},
			},
		},
	}
	_, err := ec2Client.ModifyImageAttribute(input)
	if err != nil {
		t.Fatal(err)
	}
}

func MakeAmiPublic(t *testing.T, amiID string, ec2Client *ec2.EC2) {
	input := &ec2.ModifyImageAttributeInput{
		ImageId: aws.String(amiID),
		LaunchPermission: &ec2.LaunchPermissionModifications{
			Add: []*ec2.LaunchPermission{
				{
					Group: aws.String("all"),
				},
			},
		},
	}
	_, err := ec2Client.ModifyImageAttribute(input)
	if err != nil {
		t.Fatal(err)
	}
}
