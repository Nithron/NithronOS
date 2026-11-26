# n8n - Workflow Automation

Free and open fair-code licensed workflow automation tool.

## Overview

n8n is an extendable workflow automation tool that enables you to connect anything to everything. With n8n, you can create complex workflows to automate tasks across different services and APIs. It features a visual workflow editor and supports a wide range of integrations.

## Features

- **Visual Workflow Editor**: Design workflows with a drag-and-drop interface
- **200+ Integrations**: Connect to popular services like Slack, GitHub, Google Sheets, and more
- **Custom Functions**: Write JavaScript code for custom logic
- **Webhooks**: Trigger workflows via HTTP webhooks
- **Scheduled Execution**: Run workflows on a schedule
- **Error Handling**: Built-in error handling and retry mechanisms
- **Self-hosted**: Full control over your data and workflows

## Configuration

### Required Settings

- **Username**: Username for accessing the n8n web interface
- **Password**: Password for accessing the n8n web interface (minimum 8 characters)
- **Database Password**: Password for the PostgreSQL database
- **Encryption Key**: Key to encrypt sensitive data in the database (minimum 16 characters)

### Optional Settings

- **Web Port**: Port to expose n8n on (default: 5678)
- **n8n Version**: Specific version to install (default: latest)
- **Database User**: PostgreSQL username (default: n8n)
- **Database Name**: PostgreSQL database name (default: n8n)
- **PostgreSQL Version**: PostgreSQL version to use (default: 15-alpine)
- **n8n Host**: Hostname for webhooks (default: localhost)
- **Protocol**: HTTP or HTTPS (default: http)
- **Webhook Base URL**: Full URL for webhook endpoints
- **Execution Process**: How workflows are executed (main or own process)
- **Timezone**: Container timezone (default: UTC)

## Storage

n8n stores data in the following locations:

- `/home/node/.n8n`: n8n configuration, credentials, and workflow data
- `/files`: File storage for workflow executions
- PostgreSQL database: Workflow definitions, execution history, and credentials

## Database

This installation includes a PostgreSQL database container that n8n uses to store:
- Workflow definitions
- Execution history
- Encrypted credentials
- User settings

The database is automatically configured and managed alongside n8n.

## Security Considerations

- **Encryption Key**: Keep your encryption key safe - it's used to encrypt all credentials
- **Basic Auth**: Enabled by default to protect the web interface
- **Database Password**: Use a strong password for the PostgreSQL database
- **HTTPS**: Use HTTPS when exposing n8n to the internet
- **Credentials**: All stored credentials are encrypted in the database

## Accessing n8n

After installation, n8n will be available at:
- Local: `http://localhost:[PORT]`
- Network: `http://[your-server-ip]:[PORT]`
- Via nOS: `https://[your-server]/apps/n8n`

Log in with the username and password you configured during installation.

## Getting Started

1. **Create Your First Workflow**:
   - Click "New Workflow" in the dashboard
   - Add nodes by clicking the "+" button
   - Connect nodes to define the flow
   - Configure each node with your credentials and settings

2. **Add Integrations**:
   - Click on a node to configure it
   - Add credentials for external services
   - Test the connection

3. **Execute Workflows**:
   - Manual: Click "Execute Workflow" button
   - Webhook: Use the webhook URL provided
   - Schedule: Set up cron expressions for automatic execution

## Webhook URLs

When configured, webhooks will be available at:
- Test: `[WEBHOOK_URL]/webhook-test/[workflow-id]`
- Production: `[WEBHOOK_URL]/webhook/[workflow-id]`

## Tips

- Start with simple workflows and gradually add complexity
- Use the expression editor for dynamic values
- Test workflows in development before activating
- Monitor execution history for debugging
- Back up your workflows regularly
- Use environment variables for sensitive configuration

## Troubleshooting

If n8n is not accessible:
1. Check both containers are running: `docker ps | grep n8n`
2. Verify database is healthy: `docker logs nos-app-n8n-db-1`
3. Check n8n logs: `docker logs nos-app-n8n-app-1`
4. Ensure all required fields were provided during installation
5. Verify port is not blocked by firewall

## Resources

- [n8n Documentation](https://docs.n8n.io/)
- [Available Integrations](https://n8n.io/integrations)
- [Workflow Examples](https://n8n.io/workflows)
