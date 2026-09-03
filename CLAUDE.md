# playground

## 目錄命名

原始碼目錄採全小寫 kebab-case。三個理由：`web/package.json` 的 `name`
欄位不允許大寫（npm 對新套件名一律拒絕）；Go module path 中的大寫字母
會在 module cache 被轉義為 `!` + 小寫；混用大小寫的路徑在不分大小寫的
macOS 與分大小寫的 CI 之間會造成 git 偵測不到變更。

此規則只適用於原始碼。交付給使用者的產物保留其顯示名稱，例如
`babywei-bakery/` 打包後是 `BabyWei Bakery.app`。

## Markdown

所有 `.md` 在 commit 前須通過檢查：

```sh
markdownlint '**/*.md'
```

規則設定在 `.markdownlint.jsonc`，每項偏離預設值的理由都記在該檔的註解
中。排除路徑在 `.markdownlintignore`。兩者也被 VS Code 的 markdownlint
擴充套件讀取，因此編輯器即時提示與命令列結果一致 —— 調整規則請改設定
檔，不要在個別檔案內加 inline 停用註解。

使用 `markdownlint` 而非 `markdownlint-cli2`：cli2 不讀
`.markdownlintignore`，會把 `node_modules` 下的相依套件文件一併掃入
（同一份測試檔，cli2 回報 4 筆違規、cli v1 回報 0 筆）。

散文採軟換行 —— 一個段落一行，不手動斷行，由編輯器負責視覺折行。
`MD013`（行長度）因此停用，理由見設定檔註解。
