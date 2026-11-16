## 镜像构建

执行当前目录下的 `build-image.sh` 可以自动构建好镜像。

## 自定义镜像名称

```conf
# Currently supports ubuntu, tlinux, openeuler
OS="ubuntu"

# Currently supports docker, ctr, podman, nerdctl
CLI_NAME="docker"

# The repository name
REPO_NAME="artifact.enflame.cn/enflame_docker_images/enflame"

# The image name
IMAGE_NAME="gcushare-scheduler-plugin"

# The image tag
TAG="latest"

# The namespace used by nerdctl, ctr
NAMESPACE="k8s.io"

```

可以根据自己的需要自定义这个镜像路径与名称。