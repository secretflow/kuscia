# 设置K3S API白名单
## 说明
K3S API白名单会过滤由envoy转发的K3S API请求，不在白名单中的请求会返回404，在白名单请求可以正常返回。

如果配置列表为空，则默认放行所有请求。

## 配置方法
配置文件位于/home/kuscia/etc/kuscia.yaml,配置项位于master.apiWhitelist节点下，由多个正则表达式组成，多个正则表达式之间是`或`的关系。

下面2个配置的效果是一样的：
```yaml
# config 1
master:
  apiWhitelist:
    - /(api(s)?(/[0-9A-Za-z_.-]+)?/v1(alpha1)?/namespaces/[0-9A-Za-z_.-]+/(pods|gateways|domainroutes|endpoints|services|events|configmaps|leases|taskresources|secrets|domaindatas|domaindatagrants|domaindatasources)(/[0-9A-Za-z_.-]+(/status$)?)?)
    - /api/v1/namespaces/[0-9A-Za-z_.-]+
    - /api/v1/nodes(/.*)?

# config 2
master:
  apiWhitelist:
    - (/(api(s)?(/[0-9A-Za-z_.-]+)?/v1(alpha1)?/namespaces/[0-9A-Za-z_.-]+/(pods|gateways|domainroutes|endpoints|services|events|configmaps|leases|taskresources|secrets|domaindatas|domaindatagrants|domaindatasources)(/[0-9A-Za-z_.-]+(/status$)?)?))|(/api/v1/namespaces/[0-9A-Za-z_.-]+)|(/api/v1/nodes(/.*)?)
```