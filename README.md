# User Activity Monitor

The user-activity-monitor blueprint is an EventBridge Serverless Lambda integration that monitors user presence and conversation activity to identify idle users and log them out of Genesys Cloud.

Read the blueprint here: https://developer.genesys.cloud/blueprints/user-activity-monitor/

## Overview

This solution provides:
- **Automated user logout** based on inactivity timeouts
- **Activity monitoring** via EventBridge integration
- **Web-based activity report** with OAuth Code Authorization/PKCE
- **Configurable timeout groups** for different user roles

## Documentation

For complete setup and configuration instructions, see [index.md](./index.md)

## Quick Start

1. **Prerequisites**: Go 1.23, NodeJS 22, AWS account, Genesys Cloud admin access
2. **Create OAuth clients** in Genesys Cloud (Client Credentials + Code Authorization)
3. **Configure** `serverless.yml` with your organization details
4. **Build**: `cd lambda && make build`
5. **Deploy**: `serverless deploy --stage dev`
6. **Store credentials** in AWS Secrets Manager
7. **Update redirect URIs** in Genesys Cloud OAuth client

## OAuth Implementation

This application uses **two separate OAuth clients**:

- **Client Credentials** (Backend) - For automated Lambda functions
- **Code Authorization with PKCE** (Frontend) - For user-facing activity report

This dual-client approach provides secure authentication for both automated backend processes and interactive user sessions.

## Migration from Implicit Grant

This implementation replaces the deprecated Implicit Grant flow with Code Authorization/PKCE, providing:
- Enhanced security with PKCE protection
- Backend token exchange
- Compliance with OAuth 2.0 best practices

## Support

- Full documentation: [index.md](./index.md)
- Genesys Cloud Developer Center: https://developer.genesys.cloud/blueprints/user-activity-monitor/
- OAuth documentation: https://developer.genesys.cloud/authorization/platform-auth/
