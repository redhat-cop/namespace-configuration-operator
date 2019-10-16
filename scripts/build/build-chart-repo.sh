echo '>> Building charts...'
find "$HELM_CHARTS_SOURCE" -mindepth 1 -maxdepth 1 -type d | while read chart; do
  version=${TRAVIS_TAG} envsubst < $chart/values.yaml.tpl > $chart/values.yaml
  version=${TRAVIS_TAG} envsubst < $chart/Chart.yaml.tpl  > $chart/Chart.yaml
  echo ">>> helm lint $chart"
  helm lint "$chart"
  chart_dest=$HELM_CHART_DEST/"`basename "$chart"`"
  echo ">>> helm package -d $chart_dest $chart"
  mkdir -p "$chart_dest"
  helm package -d "$chart_dest" "$chart"
done
echo '>>>' "helm repo index --url https://$(dirname $GITHUB_PAGES_REPO).github.io/$(basename $GITHUB_PAGES_REPO) $HELM_CHART_DEST"
helm repo index --url https://$(dirname $GITHUB_PAGES_REPO).github.io/$(basename $GITHUB_PAGES_REPO) $HELM_CHART_DEST