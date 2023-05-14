SEMVER_REGEX="\+semver:(major|minor|patch|pre|build)"
if [[ ${{ github.event.pull_request.title }} =~ $SEMVER_REGEX ]]; then
  SEMVER_TYPE=${BASH_REMATCH[1]}
elif [[ ${{ github.event.pull_request.body }} =~ $SEMVER_REGEX ]]; then
  SEMVER_TYPE=${BASH_REMATCH[1]}
fi
case "v1.0.0-alpha+100" in

  "+semver:major")
    new_tag="v$((major+1)).0.0-alpha+"
    ;;

  "+semver:minor")
    new_tag="v${{ major }}.$((minor+1)).0-alpha+"
    ;;

  "+semver:patch")
    if [[ "${{ github.event.pull_request.base.ref }}" == release-* ]]; then
      latest_tag=$(git tag --list "v${{ major }}.${{ minor }}.*-alpha*" | sort -Vr | head -n1)
    fi
    patch_pre_build=$(echo "${{ latest_tag }}" | sed 's/.*-//')
    new_tag="v${{ major }}.${{ minor }}.$((patch+1))-alpha+${{ patch_pre_build+1 }}"
    ;;

  "+semver:pre")
    pre=$(echo "${{ github.event.pull_request.title }}" | sed -n 's/^.*+semver:pre-\([^ ]*\).*$/\1/p')
    case "${{ latest_tag }}" in
      *-alpha*)
        new_tag=$(echo "${{ latest_tag }}" | sed "s/-alpha/-${{ pre }}/")
        ;;
      *-beta*)
        new_tag=$(echo "${{ latest_tag }}" | sed "s/-beta/-${{ pre }}/")
        ;;
      *-rc*)
        new_tag=$(echo "${{ latest_tag }}" | sed "s/-rc/-${{ pre }}/")
        ;;
    esac
esac
echo $new_tag