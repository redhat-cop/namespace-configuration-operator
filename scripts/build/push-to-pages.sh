git -C $GITHUB_PAGES_DIR config user.email "travis@users.noreply.github.com"
git -C $GITHUB_PAGES_DIR config user.name travis
# git -C $GITHUB_PAGES_DIR config credential.helper 'store --file ~/.travis.git.credentials'
# echo https://$GIT_USERNAME:$GIT_PASSWORD@github.com >> ~/.travis.git.credentials
git -C $GITHUB_PAGES_DIR add .
git -C $GITHUB_PAGES_DIR status
git -C $GITHUB_PAGES_DIR commit -m "Published by Travis"
git -C $GITHUB_PAGES_DIR push origin "$GITHUB_PAGES_BRANCH"