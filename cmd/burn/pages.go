package main

// secretPageHTML 是浏览器认领成功后的展示页。用 strings.Replace 注入已 HTML 转义的内容，避免 fmt 把 CSS 里的 %% 当格式符。
const secretPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>秘密已销毁 — 阅后即焚</title>
<style>
  :root {
    --paper: #f3f0e6;
    --ink: #302e28;
    --soft: #5c584e;
    --dim: #8d8879;
    --rule: #cbc5b5;
    --rule-ink: #5a554a;
    --seal: #9c3d2b;
    --serif: "Songti SC", "STSong", "Noto Serif CJK SC", "Source Han Serif SC", "SimSun", "Times New Roman", serif;
    --mono: ui-monospace, SFMono-Regular, Menlo, Consolas, "Courier New", monospace;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh;
    background: radial-gradient(90rem 60rem at 8% -10%, #f8f5ec 0%, transparent 55%), var(--paper);
    color: var(--ink);
    font-family: var(--serif); font-size: 15px; line-height: 1.95;
    -webkit-font-smoothing: antialiased;
  }
  .sheet { max-width: 1100px; margin: 0 auto; padding: 9vh 44px 100px; }
  .masthead { display: flex; justify-content: space-between; align-items: baseline; gap: 20px; }
  .brand { font-family: var(--mono); font-size: 11px; line-height: 1.6; letter-spacing: .24em; text-transform: uppercase; color: var(--dim); }
  .stamp { font-size: 12px; letter-spacing: .16em; color: var(--dim); }
  .layout { display: grid; grid-template-columns: 290px minmax(0, 1fr); gap: 68px; margin-top: 52px; }
  .aside-head { height: 52px; display: flex; align-items: center; gap: 12px; }
  .col-head { height: 52px; display: flex; align-items: flex-end; justify-content: space-between; padding-bottom: 14px; font-size: 12px; letter-spacing: .1em; color: var(--dim); }
  .seal {
    flex: none; width: 44px; height: 44px; border: 1px solid var(--seal); border-radius: 50%;
    color: var(--seal); display: flex; align-items: center; justify-content: center;
    font-size: 19px; line-height: 1; transform: rotate(-8deg);
  }
  hr.rule { border: 0; border-top: 1px solid var(--rule-ink); margin: 0; }
  hr.thin { border-top: 1px solid var(--rule); }
  h1.title { font-size: 42px; line-height: 1.3; letter-spacing: .12em; font-weight: 700; margin: 26px 0; }
  .lede { color: var(--soft); font-size: 14px; margin: 22px 0 26px; line-height: 2; }
  .status { font-size: 12px; letter-spacing: .12em; color: var(--seal); margin: 20px 0; }
  .meta { font-size: 12px; letter-spacing: .08em; color: var(--dim); margin: 18px 0 0; }
  .col-body { padding-top: 46px; }
  .letter { font-size: 21px; line-height: 2; letter-spacing: .04em; margin: 0 0 12px; }
  .letter .ash { color: var(--seal); }
  .body-text { color: var(--soft); font-size: 14.5px; margin: 0 0 14px; }
  pre {
    background: rgba(48,46,40,.035);
    border-top: 1px solid var(--rule); border-bottom: 1px solid var(--rule); border-left: 2px solid var(--seal);
    padding: 24px 26px; margin: 34px 0 22px;
    white-space: pre-wrap; word-break: break-word; overflow-x: auto;
    font-family: var(--mono); font-size: 13.5px; line-height: 2; color: var(--ink);
  }
  .date { font-style: italic; font-size: 12.5px; letter-spacing: .06em; color: var(--dim); margin: 16px 0 0; }
  footer { border-top: 1px solid var(--rule); padding-top: 18px; margin-top: 44px; color: var(--dim); font-size: 12px; letter-spacing: .1em; }
  @media (max-width: 860px) {
    .sheet { padding: 6vh 22px 70px; }
    .layout { grid-template-columns: 1fr; gap: 34px; margin-top: 32px; }
    h1.title { font-size: 36px; }
    .col-body { padding-top: 28px; }
  }
</style>
</head>
<body>
<div class="sheet">
  <header class="masthead">
    <span class="brand">/ burn /</span>
    <span class="stamp">已拆封 · 不再来</span>
  </header>

  <div class="layout">
    <aside>
      <div class="aside-head">
        <span class="seal">烬</span>
        <span class="brand">Burn after reading</span>
      </div>
      <hr class="rule">
      <h1 class="title">阅后即焚</h1>
      <hr class="rule">
      <p class="lede">读过的东西，不会留下第二份。</p>
      <p class="status">● 已焚</p>
      <hr class="rule thin">
      <p class="meta">不记录　·　不备份　·　不复原</p>
    </aside>

    <main class="col">
      <div class="col-head"><span>拆封之后</span><span>此刻</span></div>
      <hr class="rule">
      <div class="col-body">
        <p class="letter">秘密已销毁 <span class="ash">·</span></p>
        <p class="body-text">下面的内容只存在这一次，刷新页面就没了。</p>
        <pre>{{CONTENT}}</pre>
        <p class="date">此页不会再来一次</p>
        <footer>请立即保存到安全的地方。</footer>
      </div>
    </main>
  </div>
</div>
</body>
</html>`

// binaryPageHTML 是非 UTF-8 / 二进制内容的兜底页。秘密已被 Claim 销毁，不可能再向服务端
// 要一次下载，所以原始字节必须内联进这一次响应（data URI），不能放指向服务端的链接。
const binaryPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>秘密已销毁 — 阅后即焚</title>
<style>
  :root {
    --paper: #f3f0e6;
    --ink: #302e28;
    --soft: #5c584e;
    --dim: #8d8879;
    --rule: #cbc5b5;
    --rule-ink: #5a554a;
    --seal: #9c3d2b;
    --serif: "Songti SC", "STSong", "Noto Serif CJK SC", "Source Han Serif SC", "SimSun", "Times New Roman", serif;
    --mono: ui-monospace, SFMono-Regular, Menlo, Consolas, "Courier New", monospace;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh;
    background: radial-gradient(90rem 60rem at 8% -10%, #f8f5ec 0%, transparent 55%), var(--paper);
    color: var(--ink);
    font-family: var(--serif); font-size: 15px; line-height: 1.95;
    -webkit-font-smoothing: antialiased;
  }
  .sheet { max-width: 1100px; margin: 0 auto; padding: 9vh 44px 100px; }
  .masthead { display: flex; justify-content: space-between; align-items: baseline; gap: 20px; }
  .brand { font-family: var(--mono); font-size: 11px; line-height: 1.6; letter-spacing: .24em; text-transform: uppercase; color: var(--dim); }
  .stamp { font-size: 12px; letter-spacing: .16em; color: var(--dim); }
  .layout { display: grid; grid-template-columns: 290px minmax(0, 1fr); gap: 68px; margin-top: 52px; }
  .aside-head { height: 52px; display: flex; align-items: center; gap: 12px; }
  .col-head { height: 52px; display: flex; align-items: flex-end; justify-content: space-between; padding-bottom: 14px; font-size: 12px; letter-spacing: .1em; color: var(--dim); }
  .seal {
    flex: none; width: 44px; height: 44px; border: 1px solid var(--seal); border-radius: 50%;
    color: var(--seal); display: flex; align-items: center; justify-content: center;
    font-size: 19px; line-height: 1; transform: rotate(-8deg);
  }
  hr.rule { border: 0; border-top: 1px solid var(--rule-ink); margin: 0; }
  hr.thin { border-top: 1px solid var(--rule); }
  h1.title { font-size: 42px; line-height: 1.3; letter-spacing: .12em; font-weight: 700; margin: 26px 0; }
  .lede { color: var(--soft); font-size: 14px; margin: 22px 0 26px; line-height: 2; }
  .status { font-size: 12px; letter-spacing: .12em; color: var(--seal); margin: 20px 0; }
  .meta { font-size: 12px; letter-spacing: .08em; color: var(--dim); margin: 18px 0 0; }
  .col-body { padding-top: 46px; }
  .letter { font-size: 21px; line-height: 2; letter-spacing: .04em; margin: 0 0 12px; }
  .letter .ash { color: var(--seal); }
  .body-text { color: var(--soft); font-size: 14.5px; margin: 0 0 14px; }
  .warn {
    background: rgba(156,61,43,.035); border: 1px solid var(--rule); border-left: 2px solid var(--seal);
    padding: 20px 24px; margin: 30px 0 28px;
  }
  .warn strong { display: block; color: var(--seal); font-size: 14px; letter-spacing: .08em; margin-bottom: 12px; }
  .warn p { color: var(--soft); font-size: 13.5px; margin: 0 0 8px; }
  .warn p:last-child { margin-bottom: 0; }
  .btn {
    display: inline-block; background: var(--ink); color: var(--paper); border: 1px solid var(--ink);
    font-family: var(--serif); font-size: 13px; font-weight: 600; line-height: 1;
    letter-spacing: .16em; padding: 15px 30px; text-decoration: none;
    transition: background .18s ease, border-color .18s ease;
  }
  .btn:hover, .btn:focus-visible { background: var(--seal); border-color: var(--seal); }
  .btn span { margin-left: 12px; letter-spacing: 0; }
  details { margin-top: 32px; color: var(--dim); font-size: 12.5px; letter-spacing: .06em; }
  summary { cursor: pointer; }
  textarea {
    display: block; width: 100%; height: 110px; margin-top: 14px; padding: 12px;
    background: rgba(48,46,40,.04); border: 1px solid var(--rule);
    color: var(--soft); resize: vertical; word-break: break-all;
    font-family: var(--mono); font-size: 11.5px; line-height: 1.7;
  }
  .date { font-style: italic; font-size: 12.5px; letter-spacing: .06em; color: var(--dim); margin: 16px 0 0; }
  footer { border-top: 1px solid var(--rule); padding-top: 18px; margin-top: 44px; color: var(--dim); font-size: 12px; letter-spacing: .1em; }
  @media (max-width: 860px) {
    .sheet { padding: 6vh 22px 70px; }
    .layout { grid-template-columns: 1fr; gap: 34px; margin-top: 32px; }
    h1.title { font-size: 36px; }
    .col-body { padding-top: 28px; }
  }
</style>
</head>
<body>
<div class="sheet">
  <header class="masthead">
    <span class="brand">/ burn /</span>
    <span class="stamp">已拆封 · 不再来</span>
  </header>

  <div class="layout">
    <aside>
      <div class="aside-head">
        <span class="seal">烬</span>
        <span class="brand">Burn after reading</span>
      </div>
      <hr class="rule">
      <h1 class="title">阅后即焚</h1>
      <hr class="rule">
      <p class="lede">读过的东西，不会留下第二份。</p>
      <p class="status">● 已焚</p>
      <hr class="rule thin">
      <p class="meta">不记录　·　不备份　·　不复原</p>
    </aside>

    <main class="col">
      <div class="col-head"><span>拆封之后</span><span>此刻</span></div>
      <hr class="rule">
      <div class="col-body">
        <p class="letter">秘密已销毁 <span class="ash">·</span></p>
        <p class="body-text">下面的内容只存在这一次，刷新页面就没了，请先保存。</p>
        <div class="warn">
          <strong>{{REASON}}（{{SIZE}} 字节）</strong>
          <p>{{HINT}}</p>
          <p>服务没有改动任何字节，原始内容已完整保留在下面。</p>
        </div>
        <a class="btn" href="{{DATA_URI}}" download="{{FILENAME}}">下载原始文件 <span>→</span></a>
        <details>
          <summary>如果下载没反应，复制 Base64 自行解码</summary>
          <textarea readonly spellcheck="false">{{BASE64}}</textarea>
        </details>
        <footer>非 UTF-8 编码的文本，下载后用对应编码（如 GBK）打开即可看到原文。</footer>
      </div>
    </main>
  </div>
</div>
</body>
</html>`
