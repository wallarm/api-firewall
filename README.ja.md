# WallarmによるオープンソースAPIファイアウォール [![Black Hat Arsenal USA 2022](https://github.com/wallarm/api-firewall/blob/main/images/BHA2022.svg?raw=true)](https://www.blackhat.com/us-22/arsenal/schedule/index.html#open-source-api-firewall-new-features--functionalities-28038)

APIファイアウォールは、[OpenAPI/Swagger](https://www.wallarm.com/what/what-is-openapi) スキーマに基づくAPIリクエストとレスポンスの検証を提供する高性能プロキシです。これはクラウドネイティブ環境でREST APIエンドポイントを保護するために設計されています。APIファイアウォールは、リクエストとレスポンスの事前定義されたAPI仕様に一致する呼び出しを許可し、それ以外のすべてを拒否することによる、ポジティブセキュリティモデルを使用してAPIハードニングを提供します。

APIファイアウォールの**主な特長**は次のとおりです：

* 悪意のあるリクエストをブロックすることでREST APIエンドポイントを保護します
* 形式が不正なAPIレスポンスをブロックしてAPIデータ侵害を停止します
* Shadow APIエンドポイントを発見します
* OAuth 2.0プロトコルベースの認証のためのJWTアクセストークンを検証します
* （新機能）APIトークン、キー、およびCookiesをブラックリストに登録します

この製品は**オープンソース**であり、DockerHubで入手でき、すでに10億回（！）ダウンロードされています。このプロジェクトを支援するためには、[リポジトリ](https://hub.docker.com/r/wallarm/api-firewall)をスターでマークできます。

## 使用事例

### ブロッキングモードでの実行
* OpenAPI 3.0仕様に一致しない悪意のあるリクエストをブロックします
* データ侵害と機密情報の露出を止めるために、形式が不正なAPIレスポンスをブロックします

### モニタリングモードでの実行
* Shadow APIやドキュメント化されていないAPIエンドポイントを発見します
* OpenAPI 3.0仕様に一致しない形式の不正なリクエストとレスポンスをログに記録します

## APIスキーマ検証およびポジティブセキュリティモデル

APIファイアウォールを開始するとき、APIファイアウォールで保護する予定のアプリケーションの[OpenAPI 3.0仕様](https://swagger.io/specification/)を提供する必要があります。開始されたAPIファイアウォールは逆方向プロキシとして動作し、リクエストとレスポンスが仕様のスキーマに一致するかどうかを検証します。

スキーマと一致しないトラフィックは、[`STDOUT（標準出力）`および`STDERR（標準エラー出力）` Dockerサービス](https://docs.docker.com/config/containers/logging/)を使用してログに記録されるか、またはブロックされます（設定されたAPIファイアウォールの操作モードによります）。ログモードで動作するとき、APIファイアウォールは、API仕様でカバーされていないがリクエストに応答するいわゆるシャドウAPIエンドポイントもログに記録します（ただし、`404`のコードを返すエンドポイントを除く）。

![API Firewall scheme](https://github.com/wallarm/api-firewall/blob/main/images/Firewall%20opensource%20-%20vertical.gif?raw=true)

[OpenAPI 3.0仕様](https://swagger.io/specification/)は対応しており、YAMLまたはJSONファイル（`.yaml`、`.yml`、`.json`ファイル拡張子）として提供する必要があります。

OpenAPI 3.0仕様を使用してトラフィック要件を設定することにより、APIファイアウォールはポジティブセキュリティモデルに依存します。

## 技術データ

[APIファイアウォールは](https://www.wallarm.com/what/the-concept-of-a-firewall)、組み込みのOpenAPI 3.0リクエストおよびレスポンスバリデータを備えたリバースプロキシとして動作します。これはGolangで書かれており、fasthttpプロキシを使用しています。プロジェクトは極限のパフォーマンスとほぼゼロの追加遅延を目指して最適化されています。

## APIファイアウォールの開始

Docker上でAPIファイアウォールをダウンロード、インストール、および開始するには、[こちらの説明](https://docs.wallarm.com/api-firewall/installation-guides/docker-container/)をご覧ください。

## デモ

APIファイアウォールを試すには、APIファイアウォールで保護された例示アプリケーションをデプロイするデモ環境を実行できます。利用可能なデモ環境は2つあります：

* [APIファイアウォールのDocker Composeによるデモ](https://github.com/wallarm/api-firewall/tree/main/demo/docker-compose)
* [APIファイアウォールのKubernetesによるデモ](https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes)

## APIファイアウォールに関連するWallarmのブログ記事

* [APIファイアウォールでShadow APIを発見する](https://lab.wallarm.com/discovering-shadow-apis-with-a-api-firewall/)
* [Wallarm APIファイアウォールが本番環境でNGINXを凌ぐ](https://lab.wallarm.com/wallarm-api-firewall-outperforms-nginx-in-a-production-environment/)
* [OSS APIFWで無料でREST APIを保護する](https://lab.wallarm.com/securing-rest-with-free-api-firewall-how-to-guide/)

## パフォーマンス

APIファイアウォールを作成する際、私たちは速度と効率を優先し、顧客が最速のAPIを持つことを確認しました。最新のテストでは、APIファイアウォールが1つのリクエストを処理するのに必要な平均時間は1.339 msであり、これはNginxよりも66%高速です：

```
API Firewall 0.6.2 with JSON validation

$ ab -c 200 -n 10000 -p ./large.json -T application/json http://127.0.0.1:8282/test/signup

Requests per second:    13005.81 [#/sec] (mean)
Time per request:       15.378 [ms] (mean)
Time per request:       0.077 [ms] (mean, across all concurrent requests)

NGINX 1.18.0 without JSON validation

$ ab -c 200 -n 10000 -p ./large.json -T application/json http://127.0.0.1/test/signup

Requests per second:    7887.76 [#/sec] (mean)
Time per request:       25.356 [ms] (mean)
Time per request:       0.127 [ms] (mean, across all concurrent requests)
```

これらのパフォーマンス結果は、APIファイアウォールのテスト中に得られたものだけではありません。他の結果やAPIファイアウォールのパフォーマンスを向上させるための方法については、[このWallarmのブログ記事](https://lab.wallarm.com/wallarm-api-firewall-outperforms-nginx-in-a-production-environment/)で詳しく説明されています。