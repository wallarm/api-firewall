# Docker Composeを使用したWallarm API Firewallのデモ

このデモは、アプリケーション [**httpbin**](https://httpbin.org/) とWallarm API Firewallをプロキシとして、**httpbin** APIを保護する形でデプロイします。両方のアプリケーションは、Docker Composeを使用して接続されたDockerコンテナ内で実行されています。

## システム要件

このデモを実行する前に、システムが以下の要件を満たしていることを確認してください：

* [Mac](https://docs.docker.com/docker-for-mac/install/)、[Windows](https://docs.docker.com/docker-for-windows/install/)、または[Linux](https://docs.docker.com/engine/install/#server)用にインストールされたDocker Engine 20.x以上
* [Docker Compose](https://docs.docker.com/compose/install/)がインストールされていること
* [Mac](https://formulae.brew.sh/formula/make)、[Windows](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download)、またはLinux（適切なパッケージ管理ユーティリティを使用）用にインストールされた**make**

## 使用されるリソース

このデモで使用されるリソースは以下の通りです：

* [**httpbin** Dockerイメージ](https://hub.docker.com/r/kennethreitz/httpbin/)
* [API Firewall Dockerイメージ](https://hub.docker.com/r/wallarm/api-firewall)

## デモコードの説明

[デモコード](https://github.com/wallarm/api-firewall/tree/main/demo/docker-compose)には、以下の設定ファイルが含まれています：

* `volumes`ディレクトリに置かれている以下のOpenAPI 3.0の仕様：
    * `httpbin.json`は、[**httpbin** OpenAPI 2.0の仕様](https://httpbin.org/spec.json)をOpenAPI 3.0の仕様形式に変換したものです。
    * `httpbin-with-constraints.json`は、追加のAPI制約が明示的に追加された**httpbin**のOpenAPI 3.0の仕様です。

    これら両方のファイルはデモデプロイメントのテストに使用されます。
* `Makefile`はDockerルーチンを定義する設定ファイルです。
* `docker-compose.yml`は、**httpbin**と[API Firewall Docker](https://docs.wallarm.com/api-firewall/installation-guides/docker-container/)イメージの設定を定義するファイルです。

## ステップ1：デモコードの実行

デモコードを実行するには：

1. デモコードを含むGitHubリポジトリをクローンします：

    ```bash
    git clone https://github.com/wallarm/api-firewall.git
    ```
2. クローンしたリポジトリの`demo/docker-compose`ディレクトリに移動します：

    ```bash
    cd api-firewall/demo/docker-compose
    ```
3. 以下のコマンドを使用してデモコードを実行します：

    ```bash
    make start
    ```

    * API Firewallによって保護されたアプリケーション**httpbin**は、http://localhost:8080で利用可能です。
    * API Firewallによって保護されていないアプリケーション**httpbin**は、http://localhost:8090で利用可能です。デモデプロイメントをテストする際に、保護されていないアプリケーションにリクエストを送信して差を知ることができます。
4. デモのテストに進みます。

## ステップ2：オリジナルのOpenAPI 3.0仕様に基づくデモのテスト

デフォルトでは、このデモはオリジナルの**httpbin** OpenAPI 3.0仕様で実行されます。このデモオプションをテストするには、以下のリクエストを使用できます：

* API Firewallが未公開のパスに送信されるリクエストをブロックすることを確認します：

    ```bash
    curl -sD - http://localhost:8080/unexposed/path
    ```

    期待されるレスポンス：

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 06:58:29 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```
* API Firewallが、整数のデータ型が必要なパラメータに文字列の値を渡すリクエストをブロックすることを確認します：

    ```bash
    curl -sD - http://localhost:8080/cache/arewfser
    ```

    期待されるレスポンス：

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 06:58:29 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

    このケースでは、API FirewallがアプリケーションをCache-Poisoned DoS攻撃から保護していることを示しています。## ステップ3：より厳密なOpenAPI 3.0仕様に基づいたデモのテスト

最初に、デモで使用されるOpenAPI 3.0仕様へのパスを更新してください。

1. `docker-compose.yml` ファイル内で、`APIFW_API_SPECS` 環境変数の値をより厳密なOpenAPI 3.0仕様(`/opt/resources/httpbin-with-constraints.json`)へのパスに置き換えます。
2. 以下のコマンドを使用してデモを再起動します。

    ```bash
    make stop
    make start
    ```

その後、このデモオプションをテストするために、以下の方法を使用できます：

* 必須のクエリーパラメータ `int` が以下の定義に一致しない場合に API Firewall がリクエストをブロックすることを確認します。

    ```json
    ...
    {
      "in": "query",
      "name": "int",
      "schema": {
        "type": "integer",
        "minimum": 10,
        "maximum": 100
      },
      "required": true
    },
    ...
    ```

    以下のリクエストを使用して定義をテストします。

    ```bash
    # 必要なクエリーパラメータが欠落したリクエスト
    curl -sD - http://localhost:8080/get

    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:08 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # intパラメータ値が有効範囲内のリクエスト
    curl -sD - http://localhost:8080/get?int=15

    # 予期される応答
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 31 May 2021 07:09:38 GMT
    Content-Type: application/json
    Content-Length: 280
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true
    ...


    # intパラメータ値が範囲外のリクエスト
    curl -sD - http://localhost:8080/get?int=5

    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:27 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # intパラメータ値が範囲外のリクエスト
    curl -sD - http://localhost:8080/get?int=1000

    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:53 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # intパラメータ値が範囲外のリクエスト
    # 潜在的な危険：8バイト整数オーバーフローはスタックドロップを引き起こす可能性があります
    curl -sD - http://localhost:8080/get?int=18446744073710000001

    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:04 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

* クエリーパラメータ `str` が以下の定義に一致しない場合に API Firewall がリクエストをブロックすることを確認します。

    ```json
    ...
    {
      "in": "query",
      "name": "str",
      "schema": {
        "type": "string",
        "pattern": "^.{1,10}-\\d{1,10}$"
      }
    },
    ...
    ```

    以下のリクエストを使用して定義をテストします（`int` パラメータは引き続き必要です）：

    ```bash
    # 定義した正規表現と一致しないstrパラメータ値のリクエスト
    curl -sD - "http://localhost:8080/get?int=15&str=fasxxx.xxxawe-6354"

    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:42 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # 定義した正規表現と一致しないstrパラメータ値のリクエスト
    curl -sD - "http://localhost:8080/get?int=15&str=faswerffa-63sss54"
    
    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:42 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # 定義した正規表現と一致するstrパラメータ値のリクエスト
    curl -sD - http://localhost:8080/get?int=15&str=ri0.2-3ur0-6354

    # 予期される応答
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 31 May 2021 07:11:03 GMT
    Content-Type: application/json
    Content-Length: 331
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true
    ...


    # 定義した正規表現と一致しないstrパラメータ値のリクエスト
    # 潜在的な危険：SQL注入
    curl -sD - 'http://localhost:8080/get?int=15&str=";SELECT%20*%20FROM%20users.credentials;"'

    # 予期される応答
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:12:04 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

## ステップ4：デモコードの停止

デモデプロイメントを停止し、環境をクリアするには、以下のコマンドを使用します。

```bash
make stop
```