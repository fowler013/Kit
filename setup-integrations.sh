#!/bin/bash

# GitHub Integrations Setup Script
# This script helps you set up all the recommended GitHub integrations

echo "🚀 Setting up GitHub Integrations for Slack AI Bot"
echo "=================================================="

# Check if GitHub CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI is required. Install it from: https://cli.github.com/"
    exit 1
fi

# Get repository info
REPO_OWNER=${1:-fowler013}
REPO_NAME=${2:-Kit}
REPO_URL="https://github.com/$REPO_OWNER/$REPO_NAME"

echo "📍 Setting up integrations for: $REPO_URL"

# Function to check if user wants to proceed
confirm() {
    read -p "$1 (y/N): " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]]
}

echo ""
echo "1️⃣ DEPENDABOT SETUP"
echo "==================="
echo "✅ Dependabot configuration is already in place (.github/dependabot.yml)"
echo "   - Automatic Go module updates every Monday"
echo "   - GitHub Actions updates"
echo "   - Docker dependency updates"
echo "   - Security patches prioritized"

echo ""
echo "2️⃣ GITHUB ACTIONS SETUP"
echo "======================="
echo "✅ GitHub Actions workflows are configured:"
echo "   - CI Pipeline (.github/workflows/ci.yml)"
echo "   - Deployment Pipeline (.github/workflows/deploy.yml)"
echo "   - Automated testing, security scanning, and builds"

echo ""
echo "3️⃣ CODECLIMATE SETUP"
echo "==================="
echo "✅ CodeClimate configuration is ready (.codeclimate.yml)"
echo ""
if confirm "🌐 Do you want to enable CodeClimate for this repository?"; then
    echo "📖 To complete CodeClimate setup:"
    echo "   1. Go to: https://codeclimate.com/github"
    echo "   2. Sign in with your GitHub account"
    echo "   3. Click 'Add a repository'"
    echo "   4. Select: $REPO_OWNER/$REPO_NAME"
    echo "   5. CodeClimate will automatically use the .codeclimate.yml config"
    echo ""
    if confirm "🔗 Open CodeClimate in browser now?"; then
        open "https://codeclimate.com/github"
    fi
fi

echo ""
echo "4️⃣ SLACK GITHUB APP SETUP"
echo "========================="
echo ""
if confirm "📱 Do you want to set up Slack notifications for GitHub?"; then
    echo "📖 To set up the Slack GitHub App:"
    echo "   1. Go to your Slack workspace"
    echo "   2. Click 'Apps' in the left sidebar"
    echo "   3. Search for 'GitHub' and install the app"
    echo "   4. Run '/github subscribe $REPO_OWNER/$REPO_NAME' in any channel"
    echo "   5. Choose which events to subscribe to:"
    echo "      - issues, pulls, commits, releases, deployments"
    echo ""
    echo "💡 For CI/CD notifications, you'll also need to:"
    echo "   1. Create a Slack Webhook URL:"
    echo "      - Go to https://api.slack.com/apps"
    echo "      - Create new app → Incoming Webhooks"
    echo "      - Copy the webhook URL"
    echo "   2. Add it as a repository secret:"
    echo "      - Go to: $REPO_URL/settings/secrets/actions"
    echo "      - Create new secret: SLACK_WEBHOOK_URL"
    echo "      - Paste your webhook URL"
    echo ""
    if confirm "🔗 Open Slack Apps directory in browser?"; then
        open "https://slack.com/apps/A01BP7R4KNY-github"
    fi
fi

echo ""
echo "5️⃣ REPOSITORY SECRETS SETUP"
echo "==========================="
echo ""
if confirm "🔐 Do you want to set up repository secrets?"; then
    echo "📖 Required secrets for full functionality:"
    echo "   Go to: $REPO_URL/settings/secrets/actions"
    echo ""
    echo "   Required secrets:"
    echo "   - SLACK_WEBHOOK_URL (for CI/CD notifications)"
    echo "   - OPENAI_API_KEY (for AI integration testing)"
    echo "   - SLACK_BOT_TOKEN (for deployment testing)"
    echo "   - SLACK_APP_TOKEN (for deployment testing)"
    echo ""
    echo "   Optional secrets:"
    echo "   - ANTHROPIC_API_KEY (for Claude integration)"
    echo "   - CODECOV_TOKEN (for coverage reporting)"
    echo ""
    if confirm "🔗 Open repository secrets page in browser?"; then
        open "$REPO_URL/settings/secrets/actions"
    fi
fi

echo ""
echo "6️⃣ GITHUB ENVIRONMENTS SETUP"
echo "============================"
echo ""
if confirm "🏗️ Do you want to set up deployment environments?"; then
    echo "📖 Setting up environments for controlled deployments:"
    echo "   Go to: $REPO_URL/settings/environments"
    echo ""
    echo "   Create these environments:"
    echo "   1. 'staging' - for testing deployments"
    echo "   2. 'production' - for live deployments (with approval required)"
    echo ""
    echo "   Configure protection rules:"
    echo "   - Production: Require reviews from maintainers"
    echo "   - Production: Wait timer (e.g., 5 minutes)"
    echo "   - Both: Restrict to main branch only"
    echo ""
    if confirm "🔗 Open environments page in browser?"; then
        open "$REPO_URL/settings/environments"
    fi
fi

echo ""
echo "✅ INTEGRATION SETUP COMPLETE!"
echo "=============================="
echo ""
echo "📋 What's been configured:"
echo "   ✅ Dependabot - Automatic dependency updates"
echo "   ✅ GitHub Actions - CI/CD pipelines"
echo "   ✅ CodeClimate - Code quality monitoring"
echo "   ✅ Slack Integration - GitHub notifications"
echo ""
echo "🎯 Next steps:"
echo "   1. Complete the manual setup steps above"
echo "   2. Push these changes to trigger your first CI run"
echo "   3. Create your first issue to test the workflow"
echo "   4. Set up branch protection rules for main branch"
echo ""
echo "🚀 Your repository is now set up with professional-grade integrations!"

# Optionally open the repository
if confirm "🔗 Open repository in browser to continue setup?"; then
    open "$REPO_URL"
fi
