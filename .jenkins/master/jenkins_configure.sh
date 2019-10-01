#!/bin/bash

plugins=( "configuration-as-code" 
	  "job-dsl" 
	  "openstack-cloud"  )

function installAllPlugins {
  URL=$1
  CREDS=$2
  for plugin in "${plugins[@]}"
  do
    echo "Installing plugin: $plugin"
    curl -XPOST $URL/pluginManager/installNecessaryPlugins -u $CREDS -d '<install plugin="'"$plugin"'@current" />'
  done
}

function postConfigYaml {
  URL=$1
  CREDS=$2
  CONFIG_FILE=$3
  if [ -z "$CONFIG_FILE" ]
    then
      echo "Usage: $0 config <jenkins url> <user:auth-token> <config-file-path>"
      return 1
  fi

  echo "Posting configuration file: $CONFIG_FILE"
  curl -XPOST $URL/configuration-as-code/apply -u $CREDS --data-binary @$CONFIG_FILE
}

case $1 in
  plugins)
    installAllPlugins $2 $3
    ;;
  config)
    postConfigYaml $2 $3 $4
    ;;
  *)
    echo "Basic usage: $0 <plugins|config> <jenkins url> <user:api-token>"
    ;;
esac
