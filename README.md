## gcushare

This repository contains gcushare, which allow you to:

- Expose Enflame GCU Memory and GCU count on the node of your cluster;
- Run Enflame GCU sharing enabled containers in your Kubernetes cluster.

### Building from Source

```
git clone https://github.com/EnflameTechnology/gcushare.git
cd gcushare
./build.sh all

# This step will generate a package under dist folder.
```

## License

gcushare is licensed under the Apache-2.0 license.

gcushare was forked from AliyunContainerService gpushare-device-plugin and gpushare-scheduler-extender, both of them are licensed under Apache-2.0, many thanks to AliyunContainerService gpushare-device-plugin and gpushare-scheduler-extender:

- gpushare-device-plugin : https://github.com/AliyunContainerService/gpushare-device-plugin
- gpushare-scheduler-extender: https://github.com/AliyunContainerService/gpushare-scheduler-extender
