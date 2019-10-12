echo GITHUB_PAGES_DIR: $GITHUB_PAGES_DIR
mkdir -p $GITHUB_PAGES_DIR
echo ">> Checking out $GITHUB_PAGES_BRANCH branch from $GITHUB_PAGES_REPO"
# git -C $GITHUB_PAGES_DIR clone -b "$GITHUB_PAGES_BRANCH" "https://github.com/$GITHUB_PAGES_REPO" .
git -C $GITHUB_PAGES_DIR clone -b "$GITHUB_PAGES_BRANCH" "git@github.com:$GITHUB_PAGES_REPO.git" .