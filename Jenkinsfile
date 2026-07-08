pipeline {
    agent any

    environment {
        API_ENV_FILE_CREDENTIALS_ID = "saku-finance-api-env"

        REGISTRY_HOST_CREDENTIALS_ID = "docker-registry-host"
        REGISTRY_USERNAME_CREDENTIALS_ID = "docker-registry-username"
        REGISTRY_PASSWORD_CREDENTIALS_ID = "docker-registry-credentials"

        DEPLOY_HOST_CREDENTIALS_ID = "ganipedia-host-ssh-server"
        DEPLOY_SSH_PORT_CREDENTIALS_ID = "ganipedia-host-ssh-port"
        DEPLOY_SSH_USER_CREDENTIALS_ID = "ganipedia-host-ssh-user"
        DEPLOY_SSH_PASSWORD_CREDENTIALS_ID = "ganipedia-host-ssh-password"
        DEPLOY_PATH_CREDENTIALS_ID = "saku-finance-api-deploy-path"

        IMAGE_NAME = "api-saku-finance"
        APP_NAME = "api-saku-finance"
        DOCKER_NETWORK = "saku-finance"
        NETWORK_ALIAS = "api-saku-finance"
        CONTAINER_PORT = "4001"
        HEALTH_PATH = "/health"
        COMPOSE_FILE = "docker-compose.prd.yaml"

        DOCKER_BUILDKIT = "1"
    }

    options {
        disableConcurrentBuilds()
        timestamps()
        buildDiscarder(logRotator(numToKeepStr: '15', artifactNumToKeepStr: '5'))
        timeout(time: 30, unit: 'MINUTES')
        skipStagesAfterUnstable()
    }

    triggers {
        pollSCM('H/2 * * * *')
    }

    stages {
        stage('Checkout') {
            when {
                branch 'main'
            }
            steps {
                checkout scm
            }
        }

        stage('Initialize') {
            when {
                branch 'main'
            }
            steps {
                script {
                    env.GIT_COMMIT_SHORT = sh(
                        returnStdout: true,
                        script: 'git rev-parse --short HEAD 2>/dev/null || echo unknown'
                    ).trim()
                    env.IMAGE_TAG = "${env.BUILD_NUMBER}-${env.GIT_COMMIT_SHORT}"
                    env.LOCAL_IMAGE = "${env.IMAGE_NAME}:${env.IMAGE_TAG}"
                    env.LOCAL_LATEST = "${env.IMAGE_NAME}:latest"

                    echo """
Build Configuration
-------------------
Branch      : ${env.BRANCH_NAME}
Commit      : ${env.GIT_COMMIT_SHORT}
Image       : ${env.IMAGE_NAME}:${env.IMAGE_TAG}
Latest      : ${env.IMAGE_NAME}:latest
Registry    : configured by Jenkins credentials
Remote      : configured by Jenkins credentials
Container   : ${APP_NAME}
Network     : ${DOCKER_NETWORK}
Alias       : ${NETWORK_ALIAS}:${CONTAINER_PORT}
"""
                }
            }
        }

        stage('Validate Configuration') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    file(credentialsId: "${API_ENV_FILE_CREDENTIALS_ID}", variable: 'SAKU_API_ENV_FILE'),
                    string(credentialsId: "${REGISTRY_HOST_CREDENTIALS_ID}", variable: 'REGISTRY'),
                    string(credentialsId: "${REGISTRY_USERNAME_CREDENTIALS_ID}", variable: 'DOCKER_USER'),
                    string(credentialsId: "${REGISTRY_PASSWORD_CREDENTIALS_ID}", variable: 'DOCKER_PASS'),
                    string(credentialsId: "${DEPLOY_HOST_CREDENTIALS_ID}", variable: 'DEPLOY_HOST'),
                    string(credentialsId: "${DEPLOY_SSH_PORT_CREDENTIALS_ID}", variable: 'DEPLOY_SSH_PORT'),
                    string(credentialsId: "${DEPLOY_SSH_USER_CREDENTIALS_ID}", variable: 'DEPLOY_SSH_USER'),
                    string(credentialsId: "${DEPLOY_SSH_PASSWORD_CREDENTIALS_ID}", variable: 'SSH_PASS'),
                    string(credentialsId: "${DEPLOY_PATH_CREDENTIALS_ID}", variable: 'DEPLOY_PATH')
                ]) {
                    sh '''
                        set -euo pipefail
                        set +x

                        for name in SAKU_API_ENV_FILE REGISTRY DOCKER_USER DOCKER_PASS DEPLOY_HOST DEPLOY_SSH_PORT DEPLOY_SSH_USER SSH_PASS DEPLOY_PATH; do
                            eval "value=\\${$name:-}"
                            if [ -z "$value" ]; then
                                echo "ERROR: required Jenkins credential value $name is empty." >&2
                                exit 1
                            fi
                        done

                        case "$REGISTRY" in
                            http://*|https://*)
                                echo "ERROR: docker-registry-host must not include http:// or https://." >&2
                                exit 1
                                ;;
                        esac

                        case "$DEPLOY_SSH_PORT" in
                            *[!0-9]*|'')
                                echo "ERROR: SSH port credential must be numeric." >&2
                                exit 1
                                ;;
                        esac

                        case "$DEPLOY_PATH" in
                            /*)
                                ;;
                            *)
                                echo "ERROR: deploy path credential must be an absolute path." >&2
                                exit 1
                                ;;
                        esac

                        if [ ! -s "$SAKU_API_ENV_FILE" ]; then
                            echo "ERROR: Jenkins secret file credential $API_ENV_FILE_CREDENTIALS_ID is empty or missing." >&2
                            exit 1
                        fi

                        get_env_value() {
                            key="$1"
                            awk -v key="$key" '
                                BEGIN { FS = "=" }
                                $1 == key || $1 == "export " key {
                                    sub(/^[^=]*=/, "", $0)
                                    sub(/\r$/, "", $0)
                                    print
                                    exit
                                }
                            ' "$SAKU_API_ENV_FILE"
                        }

                        for name in APP_PORT APP_ENV CORS_ORIGINS DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME DB_SSLMODE DB_MAX_OPEN_CONNS DB_MAX_IDLE_CONNS DB_CONN_MAX_LIFETIME REDIS_HOST REDIS_PORT REDIS_DB REDIS_POOL_SIZE JWT_SECRET JWT_TTL_HOURS AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_DEFAULT_REGION AWS_BUCKET CLAUDE_API_KEY; do
                            value="$(get_env_value "$name")"
                            if [ -z "$value" ]; then
                                echo "ERROR: required API env $name is missing in $API_ENV_FILE_CREDENTIALS_ID." >&2
                                exit 1
                            fi
                        done

                        APP_PORT_VALUE="$(get_env_value APP_PORT)"
                        if [ "$APP_PORT_VALUE" != "$CONTAINER_PORT" ]; then
                            echo "ERROR: APP_PORT in $API_ENV_FILE_CREDENTIALS_ID must be $CONTAINER_PORT for this deployment." >&2
                            exit 1
                        fi
                    '''
                }
            }
        }

        stage('Build Image') {
            when {
                branch 'main'
            }
            steps {
                sh '''
                    set -euo pipefail

                    docker build \
                        --file docker/Dockerfile \
                        --tag "$LOCAL_IMAGE" \
                        --tag "$LOCAL_LATEST" \
                        --label "org.opencontainers.image.revision=$GIT_COMMIT_SHORT" \
                        --label "org.opencontainers.image.source=$JOB_NAME" \
                        --build-arg "VERSION=$GIT_COMMIT_SHORT" \
                        --progress=plain \
                        .
                '''
            }
        }

        stage('Push Image') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    string(credentialsId: "${REGISTRY_HOST_CREDENTIALS_ID}", variable: 'REGISTRY'),
                    string(credentialsId: "${REGISTRY_USERNAME_CREDENTIALS_ID}", variable: 'DOCKER_USER'),
                    string(credentialsId: "${REGISTRY_PASSWORD_CREDENTIALS_ID}", variable: 'DOCKER_PASS')
                ]) {
                    sh '''
                        set -euo pipefail
                        set +x

                        IMAGE_FULL="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
                        IMAGE_LATEST="$REGISTRY/$IMAGE_NAME:latest"

                        docker tag "$LOCAL_IMAGE" "$IMAGE_FULL"
                        docker tag "$LOCAL_LATEST" "$IMAGE_LATEST"
                        printf '%s\n' "$DOCKER_PASS" | docker login "$REGISTRY" -u "$DOCKER_USER" --password-stdin
                        docker push "$IMAGE_FULL"
                        docker push "$IMAGE_LATEST"
                        docker logout "$REGISTRY"
                    '''
                }
            }
        }

        stage('Deploy Production') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    file(credentialsId: "${API_ENV_FILE_CREDENTIALS_ID}", variable: 'SAKU_API_ENV_FILE'),
                    string(credentialsId: "${REGISTRY_HOST_CREDENTIALS_ID}", variable: 'REGISTRY'),
                    string(credentialsId: "${REGISTRY_USERNAME_CREDENTIALS_ID}", variable: 'DOCKER_USER'),
                    string(credentialsId: "${REGISTRY_PASSWORD_CREDENTIALS_ID}", variable: 'DOCKER_PASS'),
                    string(credentialsId: "${DEPLOY_HOST_CREDENTIALS_ID}", variable: 'DEPLOY_HOST'),
                    string(credentialsId: "${DEPLOY_SSH_PORT_CREDENTIALS_ID}", variable: 'DEPLOY_SSH_PORT'),
                    string(credentialsId: "${DEPLOY_SSH_USER_CREDENTIALS_ID}", variable: 'DEPLOY_SSH_USER'),
                    string(credentialsId: "${DEPLOY_SSH_PASSWORD_CREDENTIALS_ID}", variable: 'SSH_PASS'),
                    string(credentialsId: "${DEPLOY_PATH_CREDENTIALS_ID}", variable: 'DEPLOY_PATH')
                ]) {
                    sh '''
                        set -euo pipefail
                        set +x

                        ASKPASS_FILE="$(mktemp)"
                        DEPLOY_ENV_FILE="$(mktemp)"
                        REMOTE_SECRET_DIR="/tmp/$APP_NAME.$BUILD_NUMBER"
                        IMAGE_FULL="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"

                        cleanup_local() {
                            rm -f "$ASKPASS_FILE" "$DEPLOY_ENV_FILE"
                        }
                        trap cleanup_local EXIT

                        cat > "$ASKPASS_FILE" << 'ENDASKPASS'
#!/bin/sh
printf '%s\n' "$SSH_PASS"
ENDASKPASS
                        chmod 700 "$ASKPASS_FILE"

                        {
                            cat "$SAKU_API_ENV_FILE"
                            printf '\\nIMAGE_FULL=%s\\n' "$IMAGE_FULL"
                            printf 'APP_NAME=%s\\n' "$APP_NAME"
                            printf 'DOCKER_NETWORK=%s\\n' "$DOCKER_NETWORK"
                            printf 'NETWORK_ALIAS=%s\\n' "$NETWORK_ALIAS"
                            printf 'RESTART_POLICY=unless-stopped\\n'
                        } > "$DEPLOY_ENV_FILE"
                        chmod 600 "$DEPLOY_ENV_FILE"

                        export SSH_ASKPASS="$ASKPASS_FILE"
                        export SSH_ASKPASS_REQUIRE=force
                        export DISPLAY="${DISPLAY:-:0}"

                        ssh_remote() {
                            runner="ssh"
                            if command -v setsid >/dev/null 2>&1; then
                                runner="setsid ssh"
                            fi

                            $runner \
                                -o StrictHostKeyChecking=no \
                                -o ConnectTimeout=30 \
                                -o BatchMode=no \
                                -p "$DEPLOY_SSH_PORT" \
                                "$DEPLOY_SSH_USER@$DEPLOY_HOST" "$@"
                        }

                        ssh_remote "rm -rf '$REMOTE_SECRET_DIR'; mkdir -p '$REMOTE_SECRET_DIR' '$DEPLOY_PATH'; chmod 700 '$REMOTE_SECRET_DIR'"
                        printf '%s\n' "$DOCKER_PASS" | ssh_remote "umask 077; cat > '$REMOTE_SECRET_DIR/docker.pass'"
                        printf '%s\n' "$SSH_PASS" | ssh_remote "umask 077; cat > '$REMOTE_SECRET_DIR/sudo.pass'"
                        ssh_remote "umask 077; cat > '$REMOTE_SECRET_DIR/runtime.env'" < "$DEPLOY_ENV_FILE"
                        ssh_remote "cat > '$DEPLOY_PATH/$COMPOSE_FILE'" < "$COMPOSE_FILE"

                        ssh_remote "cat > /tmp/$APP_NAME-deploy.sh" << 'REMOTE_SCRIPT'
#!/bin/sh
set -eu

docker_cmd() {
    if docker info >/dev/null 2>&1; then
        docker "$@"
        return
    fi

    if command -v sudo >/dev/null 2>&1; then
        sudo -S -p '' sh -c 'exec docker "$@"' sh "$@" < "$SUDO_PASS_FILE"
        return
    fi

    echo "ERROR: current user cannot access Docker and sudo is not available." >&2
    exit 1
}

cleanup() {
    rm -rf "$REMOTE_SECRET_DIR" "/tmp/$APP_NAME-deploy.sh"
}
trap cleanup EXIT

cd "$DEPLOY_PATH"

if ! docker_cmd network inspect "$DOCKER_NETWORK" >/dev/null 2>&1; then
    echo "Creating Docker network $DOCKER_NETWORK..."
    docker_cmd network create "$DOCKER_NETWORK" >/dev/null
fi

echo "Authenticating remote Docker host to registry..."
docker_cmd login "$REGISTRY" -u "$DOCKER_USER" --password-stdin < "$DOCKER_PASS_FILE"

PREVIOUS_IMAGE="$(docker_cmd inspect --format='{{.Config.Image}}' "$APP_NAME" 2>/dev/null || true)"
if [ -n "$PREVIOUS_IMAGE" ]; then
    printf '%s\n' "$PREVIOUS_IMAGE" > .previous_image
    echo "Previous image: $PREVIOUS_IMAGE"
fi

COMPOSE_ENV_FILE="$REMOTE_SECRET_DIR/runtime.env"

echo "Deploying $IMAGE_FULL..."
docker_cmd compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" pull
docker_cmd compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" up -d --remove-orphans

echo "Waiting for API health..."
for i in $(seq 1 15); do
    if docker_cmd exec "$APP_NAME" wget -qO- "http://127.0.0.1:$CONTAINER_PORT$HEALTH_PATH" >/dev/null 2>&1; then
        echo "Health check passed"
        docker_cmd logout "$REGISTRY" >/dev/null 2>&1 || true
        docker_cmd image prune -f >/dev/null 2>&1 || true
        exit 0
    fi

    echo "Health check attempt $i failed; retrying..."
    sleep 3
done

echo "ERROR: new API container failed health check"
docker_cmd logs --tail=160 "$APP_NAME" 2>&1 || true

if [ -n "$PREVIOUS_IMAGE" ]; then
    echo "Attempting rollback to $PREVIOUS_IMAGE..."
    IMAGE_FULL="$PREVIOUS_IMAGE" docker_cmd compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" up -d --remove-orphans || true
fi

docker_cmd logout "$REGISTRY" >/dev/null 2>&1 || true
exit 1
REMOTE_SCRIPT

                        ssh_remote "
                            chmod 700 /tmp/$APP_NAME-deploy.sh
                            REGISTRY='$REGISTRY' \
                            DOCKER_USER='$DOCKER_USER' \
                            IMAGE_FULL='$IMAGE_FULL' \
                            APP_NAME='$APP_NAME' \
                            DOCKER_NETWORK='$DOCKER_NETWORK' \
                            CONTAINER_PORT='$CONTAINER_PORT' \
                            HEALTH_PATH='$HEALTH_PATH' \
                            COMPOSE_FILE='$COMPOSE_FILE' \
                            DEPLOY_PATH='$DEPLOY_PATH' \
                            REMOTE_SECRET_DIR='$REMOTE_SECRET_DIR' \
                            DOCKER_PASS_FILE='$REMOTE_SECRET_DIR/docker.pass' \
                            SUDO_PASS_FILE='$REMOTE_SECRET_DIR/sudo.pass' \
                            /tmp/$APP_NAME-deploy.sh
                        "
                    '''
                }
            }
        }

        stage('Verify Production') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    string(credentialsId: "${DEPLOY_HOST_CREDENTIALS_ID}", variable: 'DEPLOY_HOST'),
                    string(credentialsId: "${DEPLOY_SSH_PORT_CREDENTIALS_ID}", variable: 'DEPLOY_SSH_PORT'),
                    string(credentialsId: "${DEPLOY_SSH_USER_CREDENTIALS_ID}", variable: 'DEPLOY_SSH_USER'),
                    string(credentialsId: "${DEPLOY_SSH_PASSWORD_CREDENTIALS_ID}", variable: 'SSH_PASS'),
                    string(credentialsId: "${DEPLOY_PATH_CREDENTIALS_ID}", variable: 'DEPLOY_PATH')
                ]) {
                    sh '''
                        set -euo pipefail
                        set +x

                        ASKPASS_FILE="$(mktemp)"
                        REMOTE_VERIFY_DIR="/tmp/$APP_NAME.verify.$BUILD_NUMBER"

                        cleanup_local() {
                            rm -f "$ASKPASS_FILE"
                        }
                        trap cleanup_local EXIT

                        cat > "$ASKPASS_FILE" << 'ENDASKPASS'
#!/bin/sh
printf '%s\n' "$SSH_PASS"
ENDASKPASS
                        chmod 700 "$ASKPASS_FILE"

                        export SSH_ASKPASS="$ASKPASS_FILE"
                        export SSH_ASKPASS_REQUIRE=force
                        export DISPLAY="${DISPLAY:-:0}"

                        ssh_remote() {
                            runner="ssh"
                            if command -v setsid >/dev/null 2>&1; then
                                runner="setsid ssh"
                            fi

                            $runner \
                                -o StrictHostKeyChecking=no \
                                -o ConnectTimeout=30 \
                                -o BatchMode=no \
                                -p "$DEPLOY_SSH_PORT" \
                                "$DEPLOY_SSH_USER@$DEPLOY_HOST" "$@"
                        }

                        ssh_remote "rm -rf '$REMOTE_VERIFY_DIR'; mkdir -p '$REMOTE_VERIFY_DIR'; chmod 700 '$REMOTE_VERIFY_DIR'"
                        printf '%s\n' "$SSH_PASS" | ssh_remote "umask 077; cat > '$REMOTE_VERIFY_DIR/sudo.pass'"

                        ssh_remote "cat > /tmp/$APP_NAME-verify.sh" << 'REMOTE_VERIFY'
#!/bin/sh
set -eu

docker_cmd() {
    if docker info >/dev/null 2>&1; then
        docker "$@"
        return
    fi

    if command -v sudo >/dev/null 2>&1; then
        sudo -S -p '' sh -c 'exec docker "$@"' sh "$@" < "$SUDO_PASS_FILE"
        return
    fi

    echo "ERROR: current user cannot access Docker and sudo is not available." >&2
    exit 1
}

cleanup() {
    rm -rf "$REMOTE_VERIFY_DIR" "/tmp/$APP_NAME-verify.sh"
}
trap cleanup EXIT

cd "$DEPLOY_PATH"

if ! docker_cmd ps --filter "name=^/$APP_NAME\\$" --format "{{.Names}}" | grep -q "^$APP_NAME\\$"; then
    echo "ERROR: $APP_NAME is not running"
    docker_cmd logs --tail=160 "$APP_NAME" 2>&1 || true
    exit 1
fi

if ! docker_cmd inspect "$APP_NAME" --format "{{json .NetworkSettings.Networks}}" | grep -q "\"$DOCKER_NETWORK\""; then
    echo "ERROR: $APP_NAME is not attached to Docker network $DOCKER_NETWORK"
    exit 1
fi

docker_cmd exec "$APP_NAME" wget -qO- "http://127.0.0.1:$CONTAINER_PORT$HEALTH_PATH" >/dev/null
docker_cmd ps --filter "name=^/$APP_NAME\\$"
REMOTE_VERIFY

                        ssh_remote "
                            chmod 700 /tmp/$APP_NAME-verify.sh
                            APP_NAME='$APP_NAME' \
                            DOCKER_NETWORK='$DOCKER_NETWORK' \
                            CONTAINER_PORT='$CONTAINER_PORT' \
                            HEALTH_PATH='$HEALTH_PATH' \
                            COMPOSE_FILE='$COMPOSE_FILE' \
                            DEPLOY_PATH='$DEPLOY_PATH' \
                            REMOTE_VERIFY_DIR='$REMOTE_VERIFY_DIR' \
                            SUDO_PASS_FILE='$REMOTE_VERIFY_DIR/sudo.pass' \
                            /tmp/$APP_NAME-verify.sh
                        "
                    '''
                }
            }
        }
    }

    post {
        always {
            sh '''
                docker image prune -f >/dev/null 2>&1 || true
            '''
        }
        success {
            echo "Deployment successful: ${IMAGE_NAME}:${IMAGE_TAG}"
            echo "API upstream: http://${NETWORK_ALIAS}:${CONTAINER_PORT}"
        }
        failure {
            echo "Deployment failed. Check Jenkins logs for details."
            echo "If the remote error is Docker socket permission, add the deploy user to the docker group on the production server."
        }
    }
}
