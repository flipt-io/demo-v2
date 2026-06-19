#!/bin/sh
set -e

ENDPOINT=http://floci:4566
BUCKET=git-repos

echo "Checking if S3 bucket already exists..."
if aws --endpoint-url=$ENDPOINT s3 ls s3://$BUCKET 2>/dev/null; then
  echo "Bucket already exists, skipping."
  exit 0
fi

echo "Creating S3 bucket..."
aws --endpoint-url=$ENDPOINT s3 mb s3://$BUCKET
echo "Done."
