#!/bin/bash

set -e

echo "🚀 Starting deployment of User Activity Monitor..."

# Check if AWS CLI is configured
if ! aws sts get-caller-identity &> /dev/null; then
    echo "❌ AWS CLI is not configured. Please run 'aws configure' first."
    exit 1
fi

# Check if Serverless Framework is installed
if ! command -v serverless &> /dev/null; then
    echo "❌ Serverless Framework is not installed. Installing now..."
    npm install -g serverless
fi

# Set deployment stage
STAGE=${1:-dev}
echo "📋 Deploying to stage: $STAGE"

# Build the Go binary for Lambda
echo "🔨 Building Go binary..."
cd lambda
make clean build
cd ..

# Check if build was successful
if [ ! -f "lambda/dist/monitorlambdafunction/monitorlambdafunction.zip" ]; then
    echo "❌ Failed to build Go binary"
    exit 1
fi

echo "✅ Go binary built successfully"

# Deploy using Serverless Framework
echo "☁️  Deploying to AWS..."
serverless deploy --stage $STAGE --verbose


echo "✅ Deployment completed successfully!"
