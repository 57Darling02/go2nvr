# go2nvr_vue

go2nvr 的 Vue3 Web 管理界面，面向多路视频预览、录制管理、配置编辑与系统维护。

## 核心能力

- Dashboard 多路实时预览、录制状态轮询与手动录制控制
- 录制文件浏览、预览播放、下载与删除
- 录制规则配置（预录与 trigger 参数）
- ONVIF 自动发现与测试接入（Add Stream 弹窗）
- 全局配置编辑（`/api/config`）与系统页运维能力

## 录制模块文档

- 详细设计与 API 说明：`docs/README.md`
- ONVIF 说明：`docs/onvif.md`

## 开发环境

- Node.js：`^20.19.0 || >=22.12.0`
- 包管理器：`pnpm`

## 常用命令

```sh
pnpm install
pnpm dev
pnpm run type-check
pnpm run build-only
pnpm run build
```

## 前端与后端联调

- 开发服务由 Vite 提供，默认通过 `vite.config.ts` 将 `/api` 和 `/api/ws` 代理到 `127.0.0.1:1984`
- 请确保 go2rtc/go2nvr 服务已启动并可访问对应 API
