#!/bin/sh -l

gopxl-docs -url "$DEPLOY_URL" -repository-url "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY" -repository ./ -docs "$DOCS_DIR" -dest "$DEST_DIR"