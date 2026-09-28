# FIND-L03-001：设计样本混用 Shell 语法并掩盖清理失败

## 分类与结论

- 分类：D（本次模型执行行为），当前没有证据指向 Reasonix Agent 运行时或 Desktop 投影缺陷。
- 严重度：非阻断；正式 HTML/CSS、冻结 grader、独立 HTTP 和视觉验收均通过。
- 影响：增加 4 次无效或补救性 bash 调用、7 次审批与人工等待；在 workspace 遗留 3 个临时文件，
  并遗留一个监听 8765 端口的 Python HTTP server。
- 后续：在其余设计重复中继续观察；只有跨样本稳定复现且能证明工具合同诱导时，再升级为 A/B/E 类问题。

## 现象

L03 在 Windows dev 环境中先后执行了：

1. 未指定 UTF-8 的 Path.read_text()，因系统默认 GBK 解码失败；
2. 使用 /tmp/... 的 POSIX HTTP 校验，Windows Python 将其解释为 \\tmp\\... 并找不到文件；
3. 在 Git Bash 命令中同时使用 set PID=$!、cmd.exe 风格 %PID% 与 taskkill；
4. 用 || exit 0 掩盖失败的进程终止，导致后续 Python 内容断言被跳过；
5. 用 del /q ... 2>nul || exit 0 清理文件，仍是非 Bash 语法且再次被 exit 0 掩盖。

工具回执的空输出与退出码 0 与命令本身的 || exit 0 一致，并非运行时伪造成功。

## 固定证据

- JSONL：20260813-095634.618554400-gpt-gpt-5.6-luna.jsonl，bash 调用位于 assistant
  消息行 13、17、21，相应回执位于 14–16、18–20、22。
- 遗留进程链：
  - PID 38368：Git Bash，命令行包含该次混合 Shell HTTP 校验；
  - PID 47192：Hermes Python launcher；
  - PID 11244：CPython -m http.server 8765，实际监听端口。
- 独立 Invoke-WebRequest 与 Playwright 后验确认 empty-state.html、empty-state.css 均为 HTTP 200。
- empty-state-http.html 与正式 HTML 的 SHA-256 相同：
  2527E8D852F7F5703BB06357D28581E02DBAED81676544444EC4275D1D79DDA2。
- empty-state-http.css 与正式 CSS 的 SHA-256 相同：
  BB9934F2BF3B506D1EFD5667954C3B89BE09EDFE2868F3F5C106C0C55676D8FB。
- 三个遗留文件保留在 L03 原始 workspace；服务器取证后仅停止精确 PID 11244，其父进程随之退出，
  端口 8765 已确认不再监听。

## 判定边界

- 成品质量：通过。
- Agent 执行效率：受损。
- Desktop 实时完成稳定性：通过；final 在当前会话实时显示并正常 idle，与该问题无关。
- 额外文件/副作用：存在，必须在总表单列，不能用总体通过掩盖。
