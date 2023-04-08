# Amazon Web Services (AWS)
aws ec2 describe-images --region us-west-2 --output table \
  --owners 099720109477 \
  --query "sort_by(Images, &CreationDate)[*].[CreationDate,Name,ImageId,ImageLocation,Architecture,Description]" \
  --filters "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-arm64-*"

# ImageOwnerAlias,ImageType,EnaSupport,Hypervisor

# Google Cloud Platform (GCP)
#gcloud compute images list --filter ubuntu-2204-jammy-v

# Microsoft Azure
#az vm image list --all --output table \
#  --publisher Canonical --offer 0001-com-ubuntu-server-jammy --sku 22_04-lts-gen2