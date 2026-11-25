import type { ProjectWorkspaceData } from "../api"

export const xascPresetData: ProjectWorkspaceData = {
  "project": {
    "id": "projects/xasc",
    "type": "SoftwareProject",
    "name": "xasc",
    "lifecycle": "active",
    "source": "hall-cmdb-migrated",
    "evidenceKind": "document",
    "certainty": "confirmed",
    "lastConfirmed": "2026-11-21T05:19:39Z",
    "modelVersion": 1,
    "softwareProject": {
      "purpose": "雄安数城项目交付全栈（包含 dcup 流通利用平台定制、DID 身份对接、连接器集成等）",
      "languages": [
        "Go",
        "Java",
        "TypeScript",
        "Python"
      ],
      "form": "service",
      "remoteRepository": "http://10.190.128.13/ChinaUnicomBigData/MinistryOfTechnology/platform-development/projects/external_collaboration/xiongan_chanhu_group/fe_dcup_group/plat-web-circUtil.git",
      "upstream": "http://10.190.128.13/ChinaUnicomBigData/MinistryOfTechnology/platform-development/projects/OpenSource_DIF_Group/expos-dcup",
      "maintenanceState": "active",
      "recommendedVersion": "1.0.0"
    }
  },
  "instances": [
    {
      "id": "services/xasc/bastion-web",
      "type": "SoftwareInstance",
      "name": "堡垒机 Web 门户",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "bastion-portal",
        "domains": [
          "10.16.65.115"
        ],
        "ports": [
          9443
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "10.16.65.115",
            "port": 9443
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/c0130-biznode",
      "type": "SoftwareInstance",
      "name": "业务节点 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dcup-biznode",
        "domains": [
          "172.16.21.200"
        ],
        "ports": [
          19096
        ],
        "endpoints": [
          {
            "kind": "http",
            "address": "172.16.21.200",
            "port": 19096
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/c0130-conn1",
      "type": "SoftwareInstance",
      "name": "连接器1 (买方1 c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector",
        "domains": [
          "172.16.21.25"
        ],
        "ports": [
          32211,
          32111
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "172.16.21.25",
            "port": 32211
          },
          {
            "kind": "http",
            "address": "172.16.21.25",
            "port": 32111
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/c0130-conn2",
      "type": "SoftwareInstance",
      "name": "连接器2 (卖方 c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector",
        "domains": [
          "172.16.21.25"
        ],
        "ports": [
          31211,
          31111
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "172.16.21.25",
            "port": 31211
          },
          {
            "kind": "http",
            "address": "172.16.21.25",
            "port": 31111
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/c0130-minio",
      "type": "SoftwareInstance",
      "name": "MinIO 对象存储 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "minio",
        "domains": [
          "172.16.21.45"
        ],
        "ports": [
          19001
        ],
        "endpoints": [
          {
            "kind": "http",
            "address": "172.16.21.45",
            "port": 19001
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/c0130-nacos",
      "type": "SoftwareInstance",
      "name": "Nacos 配置中心 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "nacos",
        "domains": [
          "172.16.22.147"
        ],
        "ports": [
          8848
        ],
        "endpoints": [
          {
            "kind": "http",
            "address": "172.16.22.147",
            "port": 8848
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/c0130-region1",
      "type": "SoftwareInstance",
      "name": "区域节点1 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dcup-region",
        "domains": [
          "172.16.21.200"
        ],
        "ports": [
          19091
        ],
        "endpoints": [
          {
            "kind": "http",
            "address": "172.16.21.200",
            "port": 19091
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/did-service",
      "type": "SoftwareInstance",
      "name": "DID 身份服务",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "did-service",
        "domains": [
          "10.124.134.68"
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/plat-web",
      "type": "SoftwareInstance",
      "name": "前端仓库 plat-web-circUtil",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "plat-web-circUtil",
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/prod-connector",
      "type": "SoftwareInstance",
      "name": "连接器 (生产)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector",
        "domains": [
          "www.xactds.com"
        ],
        "ports": [
          39090
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "www.xactds.com",
            "port": 39090
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/prod-dashboard",
      "type": "SoftwareInstance",
      "name": "大屏 Web (生产)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dashboard",
        "domains": [
          "www.xadep.com"
        ],
        "ports": [
          30081
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "www.xadep.com",
            "port": 30081
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/prod-region-web",
      "type": "SoftwareInstance",
      "name": "区域功能节点 (生产)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dcup-region-web",
        "domains": [
          "www.xactds.com"
        ],
        "ports": [
          39090
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "www.xactds.com",
            "port": 39090
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/prod-tdrs-web",
      "type": "SoftwareInstance",
      "name": "流通利用 Web (生产)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dcup-tdrs",
        "domains": [
          "www.xactds.com"
        ],
        "ports": [
          443
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "www.xactds.com",
            "port": 443
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/sslvpn",
      "type": "SoftwareInstance",
      "name": "SSL VPN 网关",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "SSL-VPN",
        "domains": [
          "103.131.138.20"
        ],
        "ports": [
          64443
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "103.131.138.20",
            "port": 64443
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-biz-mysql",
      "type": "SoftwareInstance",
      "name": "业务节点 MySQL (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "mysql",
        "domains": [
          "121.29.88.190"
        ],
        "ports": [
          39093
        ],
        "endpoints": [
          {
            "kind": "tcp",
            "address": "121.29.88.190",
            "port": 39093
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-biz-nacos",
      "type": "SoftwareInstance",
      "name": "业务节点 Nacos (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "nacos",
        "domains": [
          "121.29.88.190"
        ],
        "ports": [
          39091
        ],
        "endpoints": [
          {
            "kind": "http",
            "address": "121.29.88.190",
            "port": 39091
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-biz-shenyu",
      "type": "SoftwareInstance",
      "name": "业务节点 ShenYu (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "shenyu",
        "domains": [
          "121.29.88.190"
        ],
        "ports": [
          39095
        ],
        "endpoints": [
          {
            "kind": "http",
            "address": "121.29.88.190",
            "port": 39095
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-biz-web",
      "type": "SoftwareInstance",
      "name": "业务节点 Web (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dcup-biz-web",
        "domains": [
          "121.29.88.190"
        ],
        "ports": [
          39092
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "121.29.88.190",
            "port": 39092
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-conn1",
      "type": "SoftwareInstance",
      "name": "连接器1 (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector",
        "domains": [
          "121.29.5.195"
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-conn1-nacos",
      "type": "SoftwareInstance",
      "name": "连接器1 Nacos (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector-nacos",
        "domains": [
          "121.29.5.195"
        ],
        "ports": [
          18881
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "121.29.5.195",
            "port": 18881
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-conn1-web",
      "type": "SoftwareInstance",
      "name": "连接器1 Web (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector-web",
        "domains": [
          "121.29.5.195"
        ],
        "ports": [
          32111
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "121.29.5.195",
            "port": 32111
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-conn2",
      "type": "SoftwareInstance",
      "name": "连接器2 (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector",
        "domains": [
          "121.29.5.59"
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-conn2-nacos",
      "type": "SoftwareInstance",
      "name": "连接器2 Nacos (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector-nacos",
        "domains": [
          "121.29.5.59"
        ],
        "ports": [
          18881
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "121.29.5.59",
            "port": 18881
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-conn2-web",
      "type": "SoftwareInstance",
      "name": "连接器2 Web (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "connector-web",
        "domains": [
          "121.29.5.59"
        ],
        "ports": [
          32111
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "121.29.5.59",
            "port": 32111
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    },
    {
      "id": "services/xasc/test-region-web",
      "type": "SoftwareInstance",
      "name": "区域功能节点 Web (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "softwareInstance": {
        "productName": "dcup-region-web",
        "domains": [
          "121.29.88.190"
        ],
        "ports": [
          39090
        ],
        "endpoints": [
          {
            "kind": "https",
            "address": "121.29.88.190",
            "port": 39090
          }
        ],
        "deploymentState": "deployed",
        "operationalState": "active"
      }
    }
  ],
  "machines": [
    {
      "id": "machine/xasc/bastion",
      "type": "Machine",
      "name": "堡垒机",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "machine": {
        "machineKind": "shared",
        "hostname": "xasc-bastion",
        "ips": [
          "10.16.65.115"
        ],
        "status": "active",
        "accessMethod": "https://10.16.65.115:9443",
        "responsibilityBoundary": "雄安数城云平台堡垒机入口，经 SSL VPN 访问"
      }
    },
    {
      "id": "machine/xasc/host-did",
      "type": "Machine",
      "name": "DID 机器",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "machine": {
        "machineKind": "rig",
        "hostname": "xasc-did-host",
        "ips": [
          "10.124.134.68"
        ],
        "status": "active",
        "responsibilityBoundary": "一体化环境 DID 身份微服务承载主机"
      }
    },
    {
      "id": "machine/xasc/prod-shenyu-admin",
      "type": "Machine",
      "name": "流通利用物理服务器 (生产)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "machine": {
        "machineKind": "rig",
        "hostname": "xasc-prod-server",
        "ips": [
          "10.124.134.19"
        ],
        "status": "active",
        "accessMethod": "ssh root@10.124.134.19",
        "responsibilityBoundary": "生产环境流通利用与大屏物理服务器"
      }
    },
    {
      "id": "machine/xasc/test-biz-bizsvr",
      "type": "Machine",
      "name": "流通利用平台-业务节点主机 (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "machine": {
        "machineKind": "rig",
        "hostname": "xasc-test-biznode",
        "ips": [
          "121.29.5.118",
          "10.26.6.245"
        ],
        "status": "active",
        "accessMethod": "ssh root@121.29.5.118",
        "responsibilityBoundary": "测试环境业务节点全栈主机，承载 Web、Nacos、ShenYu 与 MySQL"
      }
    }
  ],
  "credentials": [
    {
      "id": "credentials/xasc/c0130-conn1-login",
      "type": "AccountCredential",
      "name": "连接器1 登录账号 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "liuhaichao",
        "credentialType": "web_login",
        "purpose": "0130 连接器1 买方系统登录凭据",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/c0130-conn1-login",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/c0130-conn1-trade",
      "type": "AccountCredential",
      "name": "连接器1 流通利用账号 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "17631780025",
        "credentialType": "web_login",
        "purpose": "0130 连接器1 买方流通利用账号",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/c0130-conn1-trade",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/c0130-minio-admin",
      "type": "AccountCredential",
      "name": "MinIO 管理员 (c0130)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "dap-minio-admin",
        "credentialType": "access_key",
        "purpose": "0130 连接器环境 MinIO 存储认证",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/c0130-minio-admin",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/cred-bastion-prod",
      "type": "AccountCredential",
      "name": "堡垒机账号 (生产)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "szcs_srtgcpt_I_02_BH",
        "credentialType": "password",
        "purpose": "堡垒机生产环境登录认证",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/cred-bastion-prod",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/cred-bastion-test",
      "type": "AccountCredential",
      "name": "堡垒机账号 (一体化)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "szcs_srtgcpt_I_BH",
        "credentialType": "password",
        "purpose": "堡垒机一体化环境登录认证",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/cred-bastion-test",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/cred-test-biz-ssh",
      "type": "AccountCredential",
      "name": "业务节点 SSH 凭据",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "root",
        "credentialType": "ssh_password",
        "purpose": "业务节点主机 SSH 运维管理",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/cred-test-biz-ssh",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/prod-dashboard-admin",
      "type": "AccountCredential",
      "name": "生产大屏管理员",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "admin",
        "credentialType": "web_login",
        "purpose": "生产大屏展示系统登录账号",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/prod-dashboard-admin",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/prod-shenyu-ssh",
      "type": "AccountCredential",
      "name": "流通利用物理服务器 SSH",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "root",
        "credentialType": "ssh_password",
        "purpose": "生产环境物理服务器 SSH 凭据",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/prod-shenyu-ssh",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-biz-acct-admin",
      "type": "AccountCredential",
      "name": "平台管理员 (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "tenant",
        "credentialType": "web_login",
        "purpose": "测试环境平台管理员主账号",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-biz-acct-admin",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-biz-acct-buy",
      "type": "AccountCredential",
      "name": "购买方法人 (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "13520136102",
        "credentialType": "web_login",
        "purpose": "测试环境购买方企业法人账号 (6102)",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-biz-acct-buy",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-biz-acct-sales",
      "type": "AccountCredential",
      "name": "销售方法人 (测试)",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "13520136101",
        "credentialType": "web_login",
        "purpose": "测试环境销售方企业法人账号 (6101)",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-biz-acct-sales",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-biz-mysql",
      "type": "AccountCredential",
      "name": "业务节点 MySQL 账号",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "deployer",
        "credentialType": "db_credential",
        "purpose": "测试环境业务数据库连接凭据",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-biz-mysql",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-biz-nacos",
      "type": "AccountCredential",
      "name": "业务节点 Nacos 账号",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "nacos",
        "credentialType": "web_login",
        "purpose": "测试环境 Nacos 控制台管理员",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-biz-nacos",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-biz-shenyu",
      "type": "AccountCredential",
      "name": "业务节点 ShenYu 账号",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "admin",
        "credentialType": "web_login",
        "purpose": "测试环境 ShenYu 网关控制台",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-biz-shenyu",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-conn1-conf",
      "type": "AccountCredential",
      "name": "连接器1 conf_admin",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "conf_admin",
        "credentialType": "api_token",
        "purpose": "测试连接器1 管理员认证",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-conn1-conf",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-conn1-tenant",
      "type": "AccountCredential",
      "name": "连接器1 tenant",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "tenant",
        "credentialType": "api_token",
        "purpose": "测试连接器1 租户凭据",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-conn1-tenant",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-conn2-conf",
      "type": "AccountCredential",
      "name": "连接器2 conf_admin",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "conf_admin",
        "credentialType": "api_token",
        "purpose": "测试连接器2 管理员认证",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-conn2-conf",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-conn2-tenant",
      "type": "AccountCredential",
      "name": "连接器2 tenant",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "tenant",
        "credentialType": "api_token",
        "purpose": "测试连接器2 租户凭据",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-conn2-tenant",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-region-leg6100",
      "type": "AccountCredential",
      "name": "区域节点法人账号",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "13520136100",
        "credentialType": "web_login",
        "purpose": "区域功能节点法人账号 (6100)",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-region-leg6100",
        "status": "active"
      }
    },
    {
      "id": "credentials/xasc/test-region-tenant",
      "type": "AccountCredential",
      "name": "区域节点 tenant 账号",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "accountCredential": {
        "account": "tenant",
        "credentialType": "web_login",
        "purpose": "区域功能节点 tenant 账号",
        "scope": "xasc",
        "accessReference": "vaultwarden:xasc/test-region-tenant",
        "status": "active"
      }
    }
  ],
  "documents": [
    {
      "id": "docs/xasc/environments/common",
      "type": "Document",
      "name": "项目级公共资产接入与进入链说明",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "document": {
        "role": "进入链与公共基础设施",
        "authoritativeReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/common.md",
        "effective": true
      }
    },
    {
      "id": "docs/xasc/environments/connector-0130",
      "type": "Document",
      "name": "连接器 0130 环境档案与激活规范",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "document": {
        "role": "连接器 0130 区域与业务节点集成说明",
        "authoritativeReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/connector-0130.md",
        "effective": true
      }
    },
    {
      "id": "docs/xasc/environments/prod",
      "type": "Document",
      "name": "生产环境流通利用平台与大屏架构说明",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "document": {
        "role": "生产环境拓扑与服务地址",
        "authoritativeReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/prod.md",
        "effective": true
      }
    },
    {
      "id": "docs/xasc/environments/test",
      "type": "Document",
      "name": "测试环境全栈部署与连接器手册",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "document": {
        "role": "测试环境全栈服务与端口映射",
        "authoritativeReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/test.md",
        "effective": true
      }
    },
    {
      "id": "docs/xasc/environments/yitihua",
      "type": "Document",
      "name": "一体化环境 DID 机器与资产档案",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "document": {
        "role": "一体化环境部署拓扑",
        "authoritativeReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/yitihua.md",
        "effective": true
      }
    },
    {
      "id": "docs/xasc/runbook/connector-verification",
      "type": "Runbook",
      "name": "Runbook: 连接器端到端连通性与注册核验手册",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "runbook": {
        "scope": "xasc",
        "trigger": "需要通过 SSL VPN 访问堡垒机或测试连接器通信",
        "stepsReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/test.md",
        "approvalRequired": false
      }
    },
    {
      "id": "docs/xasc/runbook/vpn-bastion-entry",
      "type": "Runbook",
      "name": "Runbook: 雄安数城 VPN 与堡垒机双跳进入操作手册",
      "lifecycle": "active",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "lastConfirmed": "2026-11-21T05:19:39Z",
      "modelVersion": 1,
      "runbook": {
        "scope": "xasc",
        "trigger": "需要通过 SSL VPN 访问堡垒机或测试连接器通信",
        "stepsReference": "file:///home/dev/git-repo/git.svc.hp.hall/hub/xasc/environments/common.md",
        "approvalRequired": false
      }
    }
  ],
  "workItems": [],
  "relations": [
    {
      "id": "rel-docs_xasc_environments_common-documents-projects_xasc",
      "from": "docs/xasc/environments/common",
      "predicate": "documents",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_environments_connector-0130-documents-projects_xasc",
      "from": "docs/xasc/environments/connector-0130",
      "predicate": "documents",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_environments_prod-documents-projects_xasc",
      "from": "docs/xasc/environments/prod",
      "predicate": "documents",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_environments_test-documents-projects_xasc",
      "from": "docs/xasc/environments/test",
      "predicate": "documents",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_environments_yitihua-documents-projects_xasc",
      "from": "docs/xasc/environments/yitihua",
      "predicate": "documents",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_runbook_connector-verification-appliesTo-projects_xasc",
      "from": "docs/xasc/runbook/connector-verification",
      "predicate": "appliesTo",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_runbook_connector-verification-appliesTo-services_xasc_test-conn1",
      "from": "docs/xasc/runbook/connector-verification",
      "predicate": "appliesTo",
      "to": "services/xasc/test-conn1",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_runbook_connector-verification-appliesTo-services_xasc_test-conn2",
      "from": "docs/xasc/runbook/connector-verification",
      "predicate": "appliesTo",
      "to": "services/xasc/test-conn2",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_runbook_vpn-bastion-entry-appliesTo-projects_xasc",
      "from": "docs/xasc/runbook/vpn-bastion-entry",
      "predicate": "appliesTo",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_runbook_vpn-bastion-entry-appliesTo-services_xasc_bastion-web",
      "from": "docs/xasc/runbook/vpn-bastion-entry",
      "predicate": "appliesTo",
      "to": "services/xasc/bastion-web",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-docs_xasc_runbook_vpn-bastion-entry-appliesTo-services_xasc_sslvpn",
      "from": "docs/xasc/runbook/vpn-bastion-entry",
      "predicate": "appliesTo",
      "to": "services/xasc/sslvpn",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_bastion-hosts-services_xasc_bastion-web",
      "from": "machine/xasc/bastion",
      "predicate": "hosts",
      "to": "services/xasc/bastion-web",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_host-did-hosts-services_xasc_did-service",
      "from": "machine/xasc/host-did",
      "predicate": "hosts",
      "to": "services/xasc/did-service",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_prod-shenyu-admin-hasCredential-credentials_xasc_prod-shenyu-ssh",
      "from": "machine/xasc/prod-shenyu-admin",
      "predicate": "hasCredential",
      "to": "credentials/xasc/prod-shenyu-ssh",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_prod-shenyu-admin-hosts-services_xasc_prod-dashboard",
      "from": "machine/xasc/prod-shenyu-admin",
      "predicate": "hosts",
      "to": "services/xasc/prod-dashboard",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_prod-shenyu-admin-hosts-services_xasc_prod-tdrs-web",
      "from": "machine/xasc/prod-shenyu-admin",
      "predicate": "hosts",
      "to": "services/xasc/prod-tdrs-web",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_test-biz-bizsvr-hasCredential-credentials_xasc_cred-test-biz-ssh",
      "from": "machine/xasc/test-biz-bizsvr",
      "predicate": "hasCredential",
      "to": "credentials/xasc/cred-test-biz-ssh",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_test-biz-bizsvr-hosts-services_xasc_test-biz-mysql",
      "from": "machine/xasc/test-biz-bizsvr",
      "predicate": "hosts",
      "to": "services/xasc/test-biz-mysql",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_test-biz-bizsvr-hosts-services_xasc_test-biz-nacos",
      "from": "machine/xasc/test-biz-bizsvr",
      "predicate": "hosts",
      "to": "services/xasc/test-biz-nacos",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_test-biz-bizsvr-hosts-services_xasc_test-biz-shenyu",
      "from": "machine/xasc/test-biz-bizsvr",
      "predicate": "hosts",
      "to": "services/xasc/test-biz-shenyu",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_test-biz-bizsvr-hosts-services_xasc_test-biz-web",
      "from": "machine/xasc/test-biz-bizsvr",
      "predicate": "hosts",
      "to": "services/xasc/test-biz-web",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-machine_xasc_test-biz-bizsvr-hosts-services_xasc_test-region-web",
      "from": "machine/xasc/test-biz-bizsvr",
      "predicate": "hosts",
      "to": "services/xasc/test-region-web",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_bastion-web-hasCredential-credentials_xasc_cred-bastion-prod",
      "from": "services/xasc/bastion-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/cred-bastion-prod",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_bastion-web-hasCredential-credentials_xasc_cred-bastion-test",
      "from": "services/xasc/bastion-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/cred-bastion-test",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_bastion-web-implementedFrom-projects_xasc",
      "from": "services/xasc/bastion-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-biznode-implementedFrom-projects_xasc",
      "from": "services/xasc/c0130-biznode",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-conn1-hasCredential-credentials_xasc_c0130-conn1-login",
      "from": "services/xasc/c0130-conn1",
      "predicate": "hasCredential",
      "to": "credentials/xasc/c0130-conn1-login",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-conn1-hasCredential-credentials_xasc_c0130-conn1-trade",
      "from": "services/xasc/c0130-conn1",
      "predicate": "hasCredential",
      "to": "credentials/xasc/c0130-conn1-trade",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-conn1-implementedFrom-projects_xasc",
      "from": "services/xasc/c0130-conn1",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-conn2-implementedFrom-projects_xasc",
      "from": "services/xasc/c0130-conn2",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-minio-hasCredential-credentials_xasc_c0130-minio-admin",
      "from": "services/xasc/c0130-minio",
      "predicate": "hasCredential",
      "to": "credentials/xasc/c0130-minio-admin",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-minio-implementedFrom-projects_xasc",
      "from": "services/xasc/c0130-minio",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-nacos-implementedFrom-projects_xasc",
      "from": "services/xasc/c0130-nacos",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_c0130-region1-implementedFrom-projects_xasc",
      "from": "services/xasc/c0130-region1",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_did-service-implementedFrom-projects_xasc",
      "from": "services/xasc/did-service",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_plat-web-implementedFrom-projects_xasc",
      "from": "services/xasc/plat-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_prod-connector-implementedFrom-projects_xasc",
      "from": "services/xasc/prod-connector",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_prod-dashboard-hasCredential-credentials_xasc_prod-dashboard-admin",
      "from": "services/xasc/prod-dashboard",
      "predicate": "hasCredential",
      "to": "credentials/xasc/prod-dashboard-admin",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_prod-dashboard-implementedFrom-projects_xasc",
      "from": "services/xasc/prod-dashboard",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_prod-region-web-implementedFrom-projects_xasc",
      "from": "services/xasc/prod-region-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_prod-tdrs-web-implementedFrom-projects_xasc",
      "from": "services/xasc/prod-tdrs-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_sslvpn-implementedFrom-projects_xasc",
      "from": "services/xasc/sslvpn",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-mysql-hasCredential-credentials_xasc_test-biz-mysql",
      "from": "services/xasc/test-biz-mysql",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-biz-mysql",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-mysql-implementedFrom-projects_xasc",
      "from": "services/xasc/test-biz-mysql",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-nacos-hasCredential-credentials_xasc_test-biz-nacos",
      "from": "services/xasc/test-biz-nacos",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-biz-nacos",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-nacos-implementedFrom-projects_xasc",
      "from": "services/xasc/test-biz-nacos",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-shenyu-hasCredential-credentials_xasc_test-biz-shenyu",
      "from": "services/xasc/test-biz-shenyu",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-biz-shenyu",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-shenyu-implementedFrom-projects_xasc",
      "from": "services/xasc/test-biz-shenyu",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-web-hasCredential-credentials_xasc_test-biz-acct-admin",
      "from": "services/xasc/test-biz-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-biz-acct-admin",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-web-hasCredential-credentials_xasc_test-biz-acct-buy",
      "from": "services/xasc/test-biz-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-biz-acct-buy",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-web-hasCredential-credentials_xasc_test-biz-acct-sales",
      "from": "services/xasc/test-biz-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-biz-acct-sales",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-biz-web-implementedFrom-projects_xasc",
      "from": "services/xasc/test-biz-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn1-hasCredential-credentials_xasc_test-conn1-conf",
      "from": "services/xasc/test-conn1",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-conn1-conf",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn1-hasCredential-credentials_xasc_test-conn1-tenant",
      "from": "services/xasc/test-conn1",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-conn1-tenant",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn1-implementedFrom-projects_xasc",
      "from": "services/xasc/test-conn1",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn1-nacos-implementedFrom-projects_xasc",
      "from": "services/xasc/test-conn1-nacos",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn1-web-implementedFrom-projects_xasc",
      "from": "services/xasc/test-conn1-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn2-hasCredential-credentials_xasc_test-conn2-conf",
      "from": "services/xasc/test-conn2",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-conn2-conf",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn2-hasCredential-credentials_xasc_test-conn2-tenant",
      "from": "services/xasc/test-conn2",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-conn2-tenant",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn2-implementedFrom-projects_xasc",
      "from": "services/xasc/test-conn2",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn2-nacos-implementedFrom-projects_xasc",
      "from": "services/xasc/test-conn2-nacos",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-conn2-web-implementedFrom-projects_xasc",
      "from": "services/xasc/test-conn2-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-region-web-hasCredential-credentials_xasc_test-region-leg6100",
      "from": "services/xasc/test-region-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-region-leg6100",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-region-web-hasCredential-credentials_xasc_test-region-tenant",
      "from": "services/xasc/test-region-web",
      "predicate": "hasCredential",
      "to": "credentials/xasc/test-region-tenant",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    },
    {
      "id": "rel-services_xasc_test-region-web-implementedFrom-projects_xasc",
      "from": "services/xasc/test-region-web",
      "predicate": "implementedFrom",
      "to": "projects/xasc",
      "source": "hall-cmdb-migrated",
      "evidenceKind": "document",
      "certainty": "confirmed",
      "recordedAt": "2026-11-21T05:19:39Z"
    }
  ]
}
