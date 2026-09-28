# TI-002：Playwright CLI 在 Windows 的非产品侧不稳定

## 分类与影响

- 分类：E（测试基础设施问题），不是 Reasonix 产品缺陷。
- 产品执行影响：无；发生在 L03、L15 和 L18 已完成后的补充浏览器视觉验收。
- 证据影响：无丢失；改为单命令执行后完成全部视觉检查。

## 现象与恢复

1. Playwright skill wrapper 与直接 npx ... playwright-cli --version 都能输出帮助或版本，但随后出现
   Node/libuv UV_HANDLE_CLOSING assertion 并返回退出码 1。
2. 打开命名 session 后，桌面尺寸、Tab、snapshot 与 screenshot 均正常。
3. 把 resize、snapshot、eval、screenshot 串在一个 PowerShell 调用中时，CLI 超过 50 秒无输出；
   操作者终止该批处理。session 未损坏。
4. 后续严格按单命令执行，320×800、深色模式、reduced-motion、console 和 request 检查全部成功。

## 后续复现

- L15：改用逐条单命令后，1440×900、390×844 和键盘焦点截图均成功；CLI 仍表现出同类 Windows
  进程退出不稳定。
- L18：`tab-new` 与后续 SVG `snapshot` 分别超过 30 秒超时，但前者实际已成功打开 SVG 标签页，后续 resize、
  screenshot 和 tab-list 均可用。`--version` 输出 `0.1.18` 后再次触发
  `UV_HANDLE_CLOSING` assertion 并返回退出码 1。
- L18 的 Desktop、focus、mobile 和独立 SVG 四张截图均已保存；这些 CLI 超时不计入模型、Desktop final 或审批耗时。
- L21：bundled Playwright package 可载入，但默认 Chromium headless shell 未安装；改用系统已安装 Chrome 后，
  1440×1000、320×800、键盘焦点、reduced-motion 与无 gradient 检查全部成功。该准备失败同样不计入
  L21 模型、Desktop final 或审批耗时。

## 防混淆

- 该等待不计入 L03 模型耗时、用户可见 final 耗时或 Reasonix wait 次数。
- Desktop 主验收早已由 Computer Use 完成；Playwright 仅为补充视觉证据。
- 输出截图复制到 visual/L03/，原始 CLI 文件仍保留在忽略目录 output/playwright/。
