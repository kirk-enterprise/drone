docker rm -f drone_master
docker run \
  --env DRONE_DEBUG=true\
  --env DRONE_GITHUB=true\
  --env DRONE_GITHUB_CLIENT=ee088f76632f097e5b58 \
  --env DRONE_GITHUB_SECRET=15354e7629df0aa8ef7a18873c8cd967c157e57f \
  --env DRONE_SECRET=123 \
  --env DRONE_OPEN=true  \
  --env DRONE_ADMIN=u2takey1  \
  --env DRONE_YAML=".kci.yml" \
  --env DATABASE_DRIVER=mysql \
  --env DATABASE_CONFIG="root:root@tcp(192.168.99.100:3306)/drone?parseTime=true"\
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
  --env DRONE_PLUGIN_PRIVILEGED="kici/kcidocker,kici/kcidocker*" \
  --volume /var/run/docker.sock:/var/run/docker.sock \
	--restart=always \
	--detach=true \
	--name=drone_agent \
  wanglei/drone agent
