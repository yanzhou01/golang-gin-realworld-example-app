#!/bin/bash
# Deploys the front-backend app to EC2 using Aurora MySQL as the database.
# Run from the front-backend-project/ directory on your local machine.
# Safe to re-run: every step is idempotent.
#
# Usage: bash deploy-aurora-ec2.sh

set -euo pipefail

EC2_HOST="ec2-13-231-221-52.ap-northeast-1.compute.amazonaws.com"
EC2_USER="ec2-user"
SSH_KEY="$HOME/Downloads/yanzhou.pem"
REMOTE_DIR="/home/ec2-user/front-backend-project"
AURORA_HOST="yanzhouw-newhire-test-source.cluster-cdximlzkzbgd.ap-northeast-1.rds.amazonaws.com"

SSH="ssh -i $SSH_KEY -o StrictHostKeyChecking=no -o BatchMode=yes $EC2_USER@$EC2_HOST"
SCP="scp -i $SSH_KEY -o StrictHostKeyChecking=no -o BatchMode=yes"

# ---------------------------------------------------------------------------
echo "=== Step 1: Verify SSH connection ==="
$SSH "echo 'SSH OK'"

# ---------------------------------------------------------------------------
echo ""
echo "=== Step 2: Ensure Docker (upstream CE) is installed ==="
# Uses get.docker.com — works on Amazon Linux, Ubuntu, Debian, RHEL, etc.
# Skips reinstall if Docker is already present. Always uses 'sudo docker'
# so there is no dependency on docker group membership or session refresh.
$SSH 'set -e
  if command -v docker &>/dev/null; then
    echo "Docker already installed: $(sudo docker --version)"
  else
    echo "Installing Docker via get.docker.com..."
    curl -fsSL https://get.docker.com | sudo sh
    sudo systemctl enable --now docker
    echo "Docker installed: $(sudo docker --version)"
  fi
  sudo docker compose version
  sudo docker buildx version'

# ---------------------------------------------------------------------------
echo ""
echo "=== Step 3: Ensure git and repos are present on EC2 ==="
$SSH "set -e
  command -v git &>/dev/null || { sudo dnf install -y git 2>/dev/null || sudo apt-get install -y git; }
  mkdir -p $REMOTE_DIR
  cd $REMOTE_DIR
  [ -d golang-gin-realworld-example-app ] || git clone https://github.com/dulao5/golang-gin-realworld-example-app
  [ -d realworld-frontend-example ]       || git clone https://github.com/dulao5/realworld-frontend-example
  echo 'Repos ready.'"

# ---------------------------------------------------------------------------
echo ""
echo "=== Step 4: Upload Aurora config files ==="
$SCP \
  golang-gin-realworld-example-app/docker-compose.aurora.yml \
  golang-gin-realworld-example-app/aurora-init.sh \
  "$EC2_USER@$EC2_HOST:$REMOTE_DIR/golang-gin-realworld-example-app/"

# ---------------------------------------------------------------------------
echo ""
echo "=== Step 5: Write db.env on EC2 ==="
# tls=skip-verify: connection is TLS-encrypted; cert not verified (matches aurora-init behaviour).
$SSH "set -e
  cat > $REMOTE_DIR/golang-gin-realworld-example-app/db.env <<'ENVEOF'
DB_DSN=conduit:conduit@tcp($AURORA_HOST:3306)/realworld?parseTime=true&charset=utf8mb4&loc=Local&tls=skip-verify
ENVEOF
  chmod +x $REMOTE_DIR/golang-gin-realworld-example-app/aurora-init.sh
  echo 'db.env written.'"

# ---------------------------------------------------------------------------
echo ""
echo "=== Step 6: Build and start Aurora stack ==="
# FRONTEND_API_URL is the EC2 public address — baked into the frontend bundle at build time
# so browsers outside the EC2 can reach the backend API.
$SSH "set -e
  export FRONTEND_API_URL=http://$EC2_HOST:8080/api
  cd $REMOTE_DIR/golang-gin-realworld-example-app
  sudo -E docker compose -f docker-compose.aurora.yml down -v 2>/dev/null || true
  sudo -E docker compose -f docker-compose.aurora.yml build backend frontend
  sudo -E docker compose -f docker-compose.aurora.yml up -d
  echo 'Waiting 40s for services to initialise...'
  sleep 40
  sudo docker compose -f docker-compose.aurora.yml ps"

# ---------------------------------------------------------------------------
echo ""
echo "=== Step 7: Verify ==="
$SSH "curl -sf http://localhost:8080/api/ping/"
echo ""
$SSH "sudo docker compose -f $REMOTE_DIR/golang-gin-realworld-example-app/docker-compose.aurora.yml logs seed 2>&1 | tail -5"

echo ""
echo "=== Deployment complete ==="
echo "  Frontend : http://$EC2_HOST:3001"
echo "  Backend  : http://$EC2_HOST:8080/api"
echo "  Demo user: alice@example.com / password123"
