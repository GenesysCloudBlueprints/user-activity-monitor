.PHONY: clean build deploy deploy-prod

clean:
	@echo "Cleaning up..."
	rm -rf dist

# Build Go binary for Lambda
build:
	@echo "Building Go binary for Lambda..."
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o dist/monitorlambdafunction/bootstrap src/monitorlambdafunction/*.go
	cd dist/monitorlambdafunction && zip monitorlambdafunction.zip bootstrap
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o dist/reaperlambdafunction/bootstrap src/reaperlambdafunction/*.go
	cd dist/reaperlambdafunction && zip reaperlambdafunction.zip bootstrap
	@echo "Build completed successfully!"

# Deploy to dev environment
deploy: build
	@echo "Deploying to dev environment..."
	cd .. && ./scripts/deploy.sh dev
	@echo "Deployment completed!"

# Deploy to production environment
deploy-prod: build
	@echo "Deploying to production environment..."
	cd .. && ./scripts/deploy.sh prod
	@echo "Production deployment completed!"
