# Popcorn Web 上流フィードバック

scrumify（PostgreSQL + OIDC 認証 + ページツリー + Tailwind）の構築中に見つけた問題の記録。v0.5.11（2026-10-06 確認）で、報告した問題はすべて修正を確認した。残っているものはない。

## 環境

| 項目 | 値 |
| --- | --- |
| `pw` / `github.com/shibukawa/popcornweb` | v0.5.11 |
| tinybind-go | v0.5.33 |
| OS | macOS (Darwin 27.0.0, arm64)、go1.27.0 |

## v0.5.11 で確認した項目

| 項目 | 確認方法 |
| --- | --- |
| 部分更新で POST フォームを含むページが 500 になる | scrumify のレイアウトにログアウトの POST フォームを戻し、`updateHeaders()` 付きの GET が 200、`navigate` 後に DOM が更新され、フォームからログアウトできた |
| 最初のページ読み込みでコンポーネントスクリプトの `setup` が呼ばれない | 検証用プロジェクトで `/` と `/stream`（`{await}` あり）を直接開き、`setup` が `{el, onSignal, teardown, props, actions}` で呼ばれた |
| `action` の無いフォームをランタイムが横取りする | `preventDefault()` したフォームを submit しても URL が変わらなかった |

scrumify に入れていた回避策は、ログアウトの POST フォームを戻して全て解消した。`data-tb-ignore` と submit のキャプチャ段階での停止は、対策としてではなく明示として残している。

## 未確認

- `examples/auction/pages/rooms/id_/page.pw.html` の `setup(el, scope)` / `scope.on(...)` が現行の `setup({ el, ... })` に更新されているか。モジュールに `examples/` が含まれないため見ていない。
