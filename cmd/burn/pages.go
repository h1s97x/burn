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
