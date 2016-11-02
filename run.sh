docker rm -f drone_master
docker run \
  --env DRONE_DEBUG=true\
  --env DRONE_GITHUB=true\
  --env DRONE_GITHUB_CLIENT=3d4582e534d4a1061898 \
  --env DRONE_GITHUB_SECRET=4db8b532e8afdcd7fbb5c3e639069c9893a0680a \
  --env DRONE_SECRET=123 \
  --env DRONE_OPEN=true  \
  --env DRONE_ADMIN=u2takey1  \
  --env DRONE_YAML=".kci.yml" \
  --env DATABASE_DRIVER=mysql \
  --env DATABASE_CONFIG="root:root@tcp(192.168.99.100:3306)/drone?parseTime=true" \
  --restart=always \
  --publish=5002:8000 \
  --detach=true \
  --name=drone_master \
  wanglei/drone


docker rm -f drone_agent 
docker run \
  --env DRONE_DEBUG=true\
  --env DRONE_SERVER=ws://192.168.99.100:5002/ws/broker \
  --env DRONE_SECRET=123 \
  --env DRONE_PLUGIN_PRIVILEGED="index.qbox.me/library/plugin*" \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --restart=always \
  --detach=true \
  --name=drone_agent \
    wanglei/drone agent
