package main

// secretPageHTML 是浏览器认领成功后的展示页。用 strings.Replace 注入已 HTML 转义的内容，避免 fmt 把 CSS 里的 %% 当格式符。
const secretPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>秘密已销毁 — burn</title>
<style>
  :root { --bg:#0b0d10; --fg:#e8ecf1; --dim:#8a94a3; --accent:#ff6b4a; --line:rgba(255,255,255,.09); }
  * { box-sizing: border-box; }
  body {
    margin:0; min-height:100vh; background:
      radial-gradient(55rem 35rem at 50% -20%, #1b1410 0%, transparent 60%), var(--bg);
    color: var(--fg); font: 15px/1.7 ui-sans-serif, -apple-system, "Segoe UI", "Noto Sans SC", sans-serif;
    padding: 8vh 20px 60px; display:flex; justify-content:center;
  }
  main { width:100%; max-width: 760px; }
  h1 { font-size: 18px; margin: 0 0 6px; font-weight: 620; }
  h1 b { color: var(--accent); }
  p.note { color: var(--dim); margin: 0 0 22px; font-size: 13.5px; }
  pre {
    background: rgba(0,0,0,.4); border:1px solid var(--line); border-radius: 12px;
    padding: 18px 20px; margin:0; overflow-x:auto; white-space: pre-wrap; word-break: break-word;
    font: 13.5px/1.7 ui-monospace, SFMono-Regular, Menlo, monospace; color:#cfd8e3;
  }
  footer { margin-top:22px; color:#4d5561; font-size:12.5px; }
</style>
</head>
<body>
<main>
  <h1>秘密已销毁 <b>·</b></h1>
  <p class="note">下面的内容只存在这一次，刷新页面就没了。</p>
  <pre>{{CONTENT}}</pre>
  <footer>请立即保存到安全的地方。</footer>
</main>
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
<title>秘密已销毁 — burn</title>
<style>
  :root { --bg:#0b0d10; --fg:#e8ecf1; --dim:#8a94a3; --accent:#ff6b4a; --line:rgba(255,255,255,.09); }
  * { box-sizing: border-box; }
  body {
    margin:0; min-height:100vh; background:
      radial-gradient(55rem 35rem at 50% -20%, #1b1410 0%, transparent 60%), var(--bg);
    color: var(--fg); font: 15px/1.7 ui-sans-serif, -apple-system, "Segoe UI", "Noto Sans SC", sans-serif;
    padding: 8vh 20px 60px; display:flex; justify-content:center;
  }
  main { width:100%; max-width: 760px; }
  h1 { font-size: 18px; margin: 0 0 6px; font-weight: 620; }
  h1 b { color: var(--accent); }
  p.note { color: var(--dim); margin: 0 0 22px; font-size: 13.5px; }
  .warn {
    background: rgba(255,107,74,.07); border:1px solid rgba(255,107,74,.35);
    border-radius:12px; padding:18px 20px; margin:0 0 20px;
  }
  .warn strong { display:block; color: var(--accent); font-size:14px; margin-bottom:8px; }
  .warn p { color:#b9c2ce; font-size:13.5px; margin:0 0 8px; }
  .warn p:last-child { margin-bottom:0; }
  .btn {
    display:inline-block; background:var(--accent); color:#16090a; text-decoration:none;
    font:600 14px/1 ui-sans-serif, inherit; padding:13px 24px; border-radius:10px;
  }
  details { margin-top:22px; color:var(--dim); font-size:13px; }
  summary { cursor:pointer; }
  textarea {
    display:block; width:100%; height:120px; margin-top:12px; padding:12px;
    background:rgba(0,0,0,.4); border:1px solid var(--line); border-radius:10px;
    color:#cfd8e3; resize:vertical; word-break:break-all;
    font:12px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace;
  }
  footer { margin-top:22px; color:#4d5561; font-size:12.5px; }
</style>
</head>
<body>
<main>
  <h1>秘密已销毁 <b>·</b></h1>
  <p class="note">下面的内容只存在这一次，刷新页面就没了，请先保存。</p>
  <div class="warn">
    <strong>{{REASON}}（{{SIZE}} 字节）</strong>
    <p>{{HINT}}</p>
    <p>服务没有改动任何字节，原始内容已完整保留在下面。</p>
  </div>
  <a class="btn" href="{{DATA_URI}}" download="{{FILENAME}}">下载原始文件</a>
  <details>
    <summary>如果下载没反应，复制 Base64 自行解码</summary>
    <textarea readonly spellcheck="false">{{BASE64}}</textarea>
  </details>
  <footer>非 UTF-8 编码的文本，下载后用对应编码（如 GBK）打开即可看到原文。</footer>
</main>
</body>
</html>`
