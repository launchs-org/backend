---
trigger: always_on
---

コードは可読性を意識してシンプルに書くこと
1文字変数は使用しないこと
コードの各行にはコメントを書いて理解しやすくすること
作業ブランチを適切に切ってそこで作業すること
model service controller を用いてわかりやすく分けること

Modelはデータベースの取得や書き込みなどの操作
Serviceはメインのビジネスロジックを書いて必要に応じてModelを呼び出す
Controllerは各種引数の取得を行なってServiceを呼び出す

ファイルも見やすい粒度で分割すること

launchs-org-full-design.md に設計図が書いています

Dockerを用いているため デバッグする時は docker compose exec app go run . 
などを用いてデバッグすること

go コマンドはコンテナにで実行してください