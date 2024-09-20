# https://github.com/elasticsearch-dump/elasticsearch-dump


LOCAL_REPO_PATH="/tmp/demo"

GITHUB_REPO_URL="https://abc123:${GITHUB_TOKEN}@github.com/${GITHUB_USER}/${GITHUB_REPO}.git"

if [ ! -d "$LOCAL_REPO_PATH" ]; then
  git clone "$GITHUB_REPO_URL" "$LOCAL_REPO_PATH"
fi

cd "$LOCAL_REPO_PATH"

# Checkout the desired branch
git checkout -b "$BRANCH"

# Remove all files in the repository except the .git directory
find . -mindepth 1 -not -path "./.git*" -delete

mkdir -p /tmp/es_backup
NEW_ELASTICSEARCH_ADDRESS="https://${ELASTICSEARCH_USERNAME}:${ELASTICSEARCH_PASSWORD}@${ELASTICSEARCH_ADDRESS#https://}"

#curl -X GET "$ELASTICSEARCH_ADDRESS/_cat/indices?format=json" -u "$ELASTICSEARCH_USERNAME:$ELASTICSEARCH_PASSWORD" --insecure | jq -r '.[].index' | while read -r index; do
#  if [ "$(echo "$index" | cut -c 1)" != "." ] && [ "${index#security-auditlog-}" = "$index" ]; then
#    NODE_TLS_REJECT_UNAUTHORIZED=0 elasticdump \
#      --input="$NEW_ELASTICSEARCH_ADDRESS/$index" \
#      --output="/tmp/es_backup/$index.settings.json" \
#      --type=settings
#    NODE_TLS_REJECT_UNAUTHORIZED=0 elasticdump \
#      --input="$NEW_ELASTICSEARCH_ADDRESS/$index" \
#      --output="/tmp/es_backup/$index.mapping.json" \
#      --type=mapping
#    NODE_TLS_REJECT_UNAUTHORIZED=0 elasticdump \
#      --input="$NEW_ELASTICSEARCH_ADDRESS/$index" \
#      --output="/tmp/es_backup/$index.json" \
#      --type=data
#  fi
#done

# Move backup files to the repository directory
#mkdir -p "$LOCAL_REPO_PATH/es_backup"
#cp -r /tmp/es_backup/* "$LOCAL_REPO_PATH/es_backup/"

mkdir -p /tmp/postgres
pg_dump --dbname="postgresql://$OCT_POSTGRESQL_USERNAME:$OCT_POSTGRESQL_PASSWORD@$OCT_POSTGRESQL_HOST:$POSTGRESQL_PORT/pennywise" > /tmp/postgres/pennywise.sql
pg_dump --dbname="postgresql://$OCT_POSTGRESQL_USERNAME:$OCT_POSTGRESQL_PASSWORD@$OCT_POSTGRESQL_HOST:$POSTGRESQL_PORT/workspace" > /tmp/postgres/workspace.sql
pg_dump --dbname="postgresql://$OCT_POSTGRESQL_USERNAME:$OCT_POSTGRESQL_PASSWORD@$OCT_POSTGRESQL_HOST:$POSTGRESQL_PORT/auth" --exclude-table api_keys --exclude-table users --exclude-table configurations > /tmp/postgres/auth.sql
pg_dump --dbname="postgresql://$POSTGRESQL_USERNAME:$POSTGRESQL_PASSWORD@$POSTGRESQL_HOST:$POSTGRESQL_PORT/migrator" > /tmp/postgres/migrator.sql
pg_dump --dbname="postgresql://$POSTGRESQL_USERNAME:$POSTGRESQL_PASSWORD@$POSTGRESQL_HOST:$POSTGRESQL_PORT/describe" > /tmp/postgres/describe.sql
pg_dump --dbname="postgresql://$POSTGRESQL_USERNAME:$POSTGRESQL_PASSWORD@$POSTGRESQL_HOST:$POSTGRESQL_PORT/onboard" > /tmp/postgres/onboard.sql
pg_dump --dbname="postgresql://$POSTGRESQL_USERNAME:$POSTGRESQL_PASSWORD@$POSTGRESQL_HOST:$POSTGRESQL_PORT/inventory" > /tmp/postgres/inventory.sql
pg_dump --dbname="postgresql://$POSTGRESQL_USERNAME:$POSTGRESQL_PASSWORD@$POSTGRESQL_HOST:$POSTGRESQL_PORT/compliance" > /tmp/postgres/compliance.sql
pg_dump --dbname="postgresql://$POSTGRESQL_USERNAME:$POSTGRESQL_PASSWORD@$POSTGRESQL_HOST:$POSTGRESQL_PORT/metadata" > /tmp/postgres/metadata.sql

mkdir -p "$LOCAL_REPO_PATH/postgres"
cp -r /tmp/postgres/* "$LOCAL_REPO_PATH/postgres/"

cd "$LOCAL_REPO_PATH"
git add .
git commit -m "Backup Elasticsearch and PostgreSQL data"
git push --set-upstream origin "$BRANCH"

rm -rf "$LOCAL_REPO_PATH/postgres"
