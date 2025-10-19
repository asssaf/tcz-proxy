# Deploying tcz-proxy to Google App Engine

This guide explains how to deploy tcz-proxy to Google App Engine Standard Environment.

## Prerequisites

1. **Google Cloud Account**: Create one at https://cloud.google.com
2. **Google Cloud SDK**: Install from https://cloud.google.com/sdk/docs/install
3. **Google Cloud Project**: Create a project in the Google Cloud Console

## Setup

### 1. Initialize Google Cloud SDK

```bash
# Login to your Google account
gcloud auth login

# Set your project ID
gcloud config set project YOUR_PROJECT_ID

# Enable App Engine API
gcloud services enable appengine.googleapis.com
```

### 2. Initialize App Engine

```bash
# Create an App Engine application (one-time setup)
gcloud app create --region=us-central
```

Choose a region close to your users. Common options:
- `us-central` (Iowa)
- `us-east1` (South Carolina)
- `europe-west` (Belgium)
- `asia-northeast1` (Tokyo)

### 3. Prepare Configuration Files

Ensure you have the following files in your project directory:

```
tcz-proxy/
├── main.go
├── go.mod
├── app.yaml
└── config.yaml
```

**Important**: Your `config.yaml` will be deployed with the app. Make sure it contains your production configuration.

## Deployment

### Basic Deployment

```bash
# Deploy the application
gcloud app deploy

# View your deployed app
gcloud app browse
```

### Deployment with Custom Configuration

You can override settings using environment variables in `app.yaml`:

```yaml
env_variables:
  CONFIG_FILE: "config.yaml"
  FOLLOW_REDIRECTS: "true"
  # Add more variables as needed
```

Or deploy with a different version:

```bash
# Deploy to a specific version
gcloud app deploy --version v1

# Deploy without promoting (for testing)
gcloud app deploy --no-promote --version test
```

## Configuration

### Environment Variables

The app reads the following environment variables (set in `app.yaml`):

- `PORT`: Automatically set by App Engine
- `CONFIG_FILE`: Path to config file (default: "config.yaml")
- `FOLLOW_REDIRECTS`: Enable redirect following ("true" or "false")

### Scaling Configuration

Edit `app.yaml` to adjust scaling:

```yaml
automatic_scaling:
  min_idle_instances: 0      # Minimum instances always running
  max_idle_instances: 1      # Maximum idle instances
  min_pending_latency: 30ms  # Minimum request wait time before new instance
  max_pending_latency: automatic
  max_concurrent_requests: 80 # Max concurrent requests per instance
```

### Instance Class

Choose an instance class based on your needs:

```yaml
# F1: 600 MHz CPU, 256 MB RAM (default, cheapest)
instance_class: F1

# F2: 1.2 GHz CPU, 512 MB RAM
instance_class: F2

# F4: 2.4 GHz CPU, 1 GB RAM
instance_class: F4

# F4_1G: 2.4 GHz CPU, 2 GB RAM
instance_class: F4_1G
```

## Testing Your Deployment

### View Logs

```bash
# Stream logs in real-time
gcloud app logs tail -s default

# View logs in Cloud Console
gcloud app logs read
```

### Test the Proxy

```bash
# Get your App Engine URL
gcloud app browse --no-launch-browser

# Test with curl (replace YOUR_APP_URL)
curl -v -x https://YOUR_APP_URL.appspot.com http://example.com/path
```

### Check Service Status

```bash
# List versions
gcloud app versions list

# Describe current version
gcloud app describe
```

## Managing Your Deployment

### Update Configuration

1. Edit `config.yaml` or `app.yaml`
2. Redeploy:
   ```bash
   gcloud app deploy
   ```

### Traffic Splitting

Split traffic between versions:

```bash
# Split traffic 50/50 between v1 and v2
gcloud app services set-traffic default --splits v1=.5,v2=.5
```

### Delete Old Versions

```bash
# List all versions
gcloud app versions list

# Delete a specific version
gcloud app versions delete v1
```

## Cost Optimization

### Free Tier

Google App Engine Standard provides a free tier:
- 28 instance hours per day
- 1 GB data transfer per day
- 1 GB Cloud Storage

### Minimize Costs

1. **Set min_idle_instances to 0**: Instances scale to zero when not in use
2. **Use F1 instance class**: Smallest and cheapest
3. **Delete old versions**: They consume resources
4. **Monitor usage**: Check Cloud Console for billing alerts

```bash
# Set a budget alert in Cloud Console
https://console.cloud.google.com/billing/budgets
```

## Troubleshooting

### Deployment Fails

```bash
# Check build logs
gcloud app logs read --service=default --limit=50

# Verify Go version
go version  # Should be 1.21 or compatible

# Verify app.yaml syntax
cat app.yaml
```

### App Not Responding

```bash
# Check if app is running
gcloud app versions list

# View recent errors
gcloud app logs read --level=error

# Check health checks
gcloud app services describe default
```

### Configuration Not Loading

1. Verify `config.yaml` is in the deployment directory
2. Check file permissions
3. View logs for configuration errors:
   ```bash
   gcloud app logs tail -s default | grep -i config
   ```

## Security Considerations

### HTTPS Only

Force HTTPS in `app.yaml`:

```yaml
handlers:
  - url: /.*
    script: auto
    secure: always  # Redirect HTTP to HTTPS
```

### Access Control

Restrict access using IAP (Identity-Aware Proxy):

```bash
# Enable IAP
gcloud iap web enable --resource-type=app-engine

# Add authorized users
gcloud iap web add-iam-policy-binding \
  --resource-type=app-engine \
  --member='user:user@example.com' \
  --role='roles/iap.httpsResourceAccessor'
```

### Secrets Management

For sensitive configuration, use Secret Manager:

```yaml
# In app.yaml
env_variables:
  DEFAULT_HOST: projects/PROJECT_ID/secrets/default-host/versions/latest
```

## Monitoring

### View Metrics

```bash
# Open Cloud Console monitoring
https://console.cloud.google.com/appengine
```

### Set Up Alerts

1. Go to Cloud Console → Monitoring → Alerting
2. Create alert for:
   - High error rate
   - Response time
   - Instance count

## Additional Resources

- [App Engine Documentation](https://cloud.google.com/appengine/docs)
- [Go Runtime Documentation](https://cloud.google.com/appengine/docs/standard/go)
- [Pricing Calculator](https://cloud.google.com/products/calculator)
- [Best Practices](https://cloud.google.com/appengine/docs/standard/go/runtime#best_practices)

## Support

For issues with:
- **tcz-proxy**: Open an issue on GitHub
- **Google App Engine**: Visit https://cloud.google.com/support
