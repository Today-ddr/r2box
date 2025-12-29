#!/bin/bash
# Release script for R2Box
# Usage: ./scripts/release.sh [major|minor|patch] [--dry-run]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
BUMP_TYPE=${1:-patch}
DRY_RUN=false

if [[ "$2" == "--dry-run" ]] || [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    if [[ "$1" == "--dry-run" ]]; then
        BUMP_TYPE="patch"
    fi
fi

# Validate bump type
if [[ ! "$BUMP_TYPE" =~ ^(major|minor|patch)$ ]]; then
    echo -e "${RED}Error: Invalid bump type '$BUMP_TYPE'. Use major, minor, or patch.${NC}"
    exit 1
fi

# Get current version from package.json
CURRENT_VERSION=$(node -p "require('./frontend/package.json').version")
echo -e "${YELLOW}Current version: v${CURRENT_VERSION}${NC}"

# Calculate new version
IFS='.' read -ra VERSION_PARTS <<< "$CURRENT_VERSION"
MAJOR=${VERSION_PARTS[0]}
MINOR=${VERSION_PARTS[1]}
PATCH=${VERSION_PARTS[2]}

case $BUMP_TYPE in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch)
        PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
echo -e "${GREEN}New version: v${NEW_VERSION}${NC}"

if $DRY_RUN; then
    echo -e "${YELLOW}[DRY RUN] Would perform the following actions:${NC}"
    echo "  1. Update frontend/package.json version to ${NEW_VERSION}"
    echo "  2. Update CHANGELOG.md with new version"
    echo "  3. Commit changes with message: 'chore(release): v${NEW_VERSION}'"
    echo "  4. Create git tag: v${NEW_VERSION}"
    echo "  5. Push commits and tags to origin"
    exit 0
fi

# Confirm release
echo ""
read -p "Proceed with release v${NEW_VERSION}? (y/N) " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${RED}Release cancelled.${NC}"
    exit 1
fi

# Check for uncommitted changes
if [[ -n $(git status --porcelain) ]]; then
    echo -e "${RED}Error: You have uncommitted changes. Please commit or stash them first.${NC}"
    exit 1
fi

# Check we're on main/master branch
CURRENT_BRANCH=$(git branch --show-current)
if [[ ! "$CURRENT_BRANCH" =~ ^(main|master)$ ]]; then
    echo -e "${YELLOW}Warning: You're not on main/master branch (current: ${CURRENT_BRANCH})${NC}"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Update package.json
echo -e "${GREEN}Updating package.json...${NC}"
cd frontend
npm version $NEW_VERSION --no-git-tag-version
cd ..

# Update CHANGELOG.md
echo -e "${GREEN}Updating CHANGELOG.md...${NC}"
TODAY=$(date +%Y-%m-%d)
sed -i.bak "s/## \[Unreleased\]/## [Unreleased]\n\n## [${NEW_VERSION}] - ${TODAY}/" CHANGELOG.md
rm -f CHANGELOG.md.bak

# Add changelog link
sed -i.bak "s|\[Unreleased\]: \(.*\)/compare/v.*\.\.\.HEAD|\[Unreleased\]: \1/compare/v${NEW_VERSION}...HEAD\n[${NEW_VERSION}]: \1/compare/v${CURRENT_VERSION}...v${NEW_VERSION}|" CHANGELOG.md
rm -f CHANGELOG.md.bak

# Commit changes
echo -e "${GREEN}Committing changes...${NC}"
git add frontend/package.json frontend/package-lock.json CHANGELOG.md
git commit -m "chore(release): v${NEW_VERSION}"

# Create tag
echo -e "${GREEN}Creating tag v${NEW_VERSION}...${NC}"
git tag -a "v${NEW_VERSION}" -m "Release v${NEW_VERSION}"

# Push
echo -e "${GREEN}Pushing to origin...${NC}"
git push origin $CURRENT_BRANCH
git push origin "v${NEW_VERSION}"

echo ""
echo -e "${GREEN}✅ Release v${NEW_VERSION} completed!${NC}"
echo ""
echo "GitHub Actions will now:"
echo "  1. Build and push Docker image with version ${NEW_VERSION}"
echo "  2. Create GitHub Release with changelog"
echo ""
echo "Monitor progress at: https://github.com/Today-ddr/r2box/actions"
