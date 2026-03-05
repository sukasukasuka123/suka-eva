# 目前项目的想法

## 当前阶段（2026/3/4）

完成了agent/base/core和agent/base/tool模块（tool模块包含了tool类规范和tool的注册中心），但没有写成微服务

## 未来的打算

agent/base/core模块注册成一个微服务，agent/base/tool模块注册成一个微服务，各个工具在micro_tool下面也注册微服务。
到时候core需要工具的时候直接从注册中心里面拿，注册中心的工具调用是用micro_tool的微服务工具
