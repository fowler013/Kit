#!/bin/bash

# GitHub Labels Setup Script for Slack AI Bot
# Run this script to create all necessary labels for the project
# Usage: ./setup-labels.sh fowler013 Kit

REPO_OWNER=${1:-fowler013}
REPO_NAME=${2:-Kit}

echo "Setting up labels for $REPO_OWNER/$REPO_NAME"

# Check if GitHub CLI is installed
if ! command -v gh &> /dev/null; then
    echo "GitHub CLI is required. Install it from: https://cli.github.com/"
    exit 1
fi

# Priority Labels
echo "Creating priority labels..."
gh label create "priority-high" --color "d73a4a" --description "High priority issues that need immediate attention" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "priority-medium" --color "fbca04" --description "Medium priority issues for next sprint" --repo "$REPO_OWNER/$REPO_NAME" || true  
gh label create "priority-low" --color "0e8a16" --description "Low priority nice-to-have features" --repo "$REPO_OWNER/$REPO_NAME" || true

# Component Labels
echo "Creating component labels..."
gh label create "ai-integration" --color "8b5cf6" --description "AI service integration and features" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "slack-api" --color "1e40af" --description "Slack API integration and features" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "devops" --color "6b7280" --description "DevOps, deployment, and infrastructure" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "testing" --color "f59e0b" --description "Testing, quality assurance, and test automation" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "documentation" --color "3b82f6" --description "Documentation improvements and updates" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "security" --color "dc2626" --description "Security features and fixes" --repo "$REPO_OWNER/$REPO_NAME" || true

# Type Labels
echo "Creating type labels..."
gh label create "enhancement" --color "10b981" --description "New features and improvements" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "feature" --color "3b82f6" --description "New feature requests" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "bug" --color "dc2626" --description "Bug reports and fixes" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "reliability" --color "ef4444" --description "Reliability and error handling improvements" --repo "$REPO_OWNER/$REPO_NAME" || true

# Effort Labels  
echo "Creating effort labels..."
gh label create "effort-small" --color "c7d2fe" --description "Small effort (1-2 days)" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "effort-medium" --color "93c5fd" --description "Medium effort (3-5 days)" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "effort-large" --color "3b82f6" --description "Large effort (1+ weeks)" --repo "$REPO_OWNER/$REPO_NAME" || true

# Status Labels
echo "Creating status labels..."
gh label create "blocked" --color "fbbf24" --description "Blocked by external dependency" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "needs-review" --color "a855f7" --description "Needs code review" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "needs-testing" --color "f97316" --description "Needs testing before merge" --repo "$REPO_OWNER/$REPO_NAME" || true

# Special Labels
echo "Creating special labels..."
gh label create "good-first-issue" --color "22c55e" --description "Good for newcomers" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "help-wanted" --color "84cc16" --description "Extra attention is needed" --repo "$REPO_OWNER/$REPO_NAME" || true
gh label create "breaking-change" --color "dc2626" --description "Changes that break backward compatibility" --repo "$REPO_OWNER/$REPO_NAME" || true

echo "✅ All labels created successfully!"
echo "🔗 View labels at: https://github.com/$REPO_OWNER/$REPO_NAME/labels"
