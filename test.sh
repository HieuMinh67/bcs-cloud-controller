PR_TITLE="[CC] Testing +semver:"
PR_BODY="Testing 4"
SEMVER_REGEX="\+semver:(major|minor|patch|pre|build)"
if ! ([[ "$PR_TITLE" =~ $SEMVER_REGEX ]] || [[ "$PR_BODY" =~ $SEMVER_REGEX ]]); then
  echo "This PR does not contain a semantic version string."
  exit 1
fi
 echo "aaa"