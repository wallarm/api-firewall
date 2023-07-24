# Kubernetesを使用したWallarm API Firewallデモ

このデモは、アプリケーション [**httpbin**](https://httpbin.org/)とWallarm API Firewallを**httpbin** APIを保護するプロキシとしてデプロイします。両方のアプリケーションは、KubernetesのDockerコンテナで動作しています。

## システム要件

このデモを実行する前に、システムが以下の要件を満たしていることを確認してください:

* [Mac](https://docs.docker.com/docker-for-mac/install/)、[Windows](https://docs.docker.com/docker-for-windows/install/)、または[Linux](https://docs.docker.com/engine/install/#server)用のDocker Engine 20.xまたはそれ以降がインストール済み
* [Docker Compose](https://docs.docker.com/compose/install/)がインストール済み
* [Mac](https://formulae.brew.sh/formula/make)、[Windows](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download)、またはLinux(適切なパッケージ管理ユーティリティを使用）用の**make**がインストール済み

このデモ環境の実行はリソースが集中して利用されます。次のリソースが利用可能であることを確認してください:

* 少なくとも2つのCPUコア
* 少なくとも6GBの揮発性メモリ

## 使用リソース

以下のリソースがこのデモで使用されます:

* [**httpbin** Dockerイメージ](https://hub.docker.com/r/kennethreitz/httpbin/)
* [API Firewall Dockerイメージ](https://hub.docker.com/r/wallarm/api-firewall)

## デモコードの説明

[デモコード](https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes)は、デプロイされた**httpbin**とAPI Firewallを含むKubernetesクラスタを実行します。

Kubernetesクラスタを実行するために、このデモはツール(**kind**)[https://kind.sigs.k8s.io/]を使用します。**kind**は、Dockerコンテナをノードとして使用して、数分でK8sクラスタを実行できるようにします。幾つかの抽象化層を使用して、**kind**とその依存関係は、Kubernetesクラスタを起動するDockerイメージにパックされます。

デモのデプロイメントは、以下のディレクトリ/ファイルを通じて設定されます:

* **httpbin** APIのOpenAPI 3.0仕様は、`volumes/helm/api-firewall.yaml` の `manifest.body` パスにあるファイルに位置しています。この仕様を使用して、API Firewallはアプリケーションアドレスに送信されるリクエストとレスポンスがアプリケーションAPIスキーマに一致するかどうかを検証します。

    この仕様は、[**httpbin**の元のAPIスキーマ](https://httpbin.org/spec.json)を定義していません。API Firewallの特性をより透明に示すために、元のOpenAPI 2.0スキーマを明示的に変換し、複雑化し、変更した仕様を `volumes/helm/api-firewall.yaml` や `manifest.body`に保存しました。
* `Makefile`は、Dockerルーチンを定義する設定ファイルです。
* `docker-compose.yml`は、一時的なKubernetesクラスタを実行するための以下の設定を定義するファイルです:

    * [`docker/Dockerfile`](https://github.com/wallarm/api-firewall/blob/main/demo/kubernetes/docker/Dockerfile)に基づいた [**kind**](https://kind.sigs.k8s.io/) ノードのビルド。
    * KubernetesとDockerサービスディスカバリを同時に提供するDNSサーバーのデプロイメント。
    * ローカルDockerレジストリと `dind` サービスのデプロイメント。
    * **httpbin** と [API Firewall Docker](https://docs.wallarm.com/api-firewall/installation-guides/docker-container/) イメージの設定。

## ステップ1：デモコードの実行

デモコードを実行するには：

1. デモコードを含むGitHubリポジトリをクローンします:

    ```bash
    git clone https://github.com/wallarm/api-firewall.git
    ```
2. クローンしたリポジトリの `demo/kubernetes` ディレクトリに移動します:

    ```bash
    cd api-firewall/demo/kubernetes
    ```
3. 以下のコマンドを使用してデモコードを実行します。このデモの実行はリソースが集中的に利用されることに注意してください。デモ環境の開始には最大で3分かかります。

    ```bash
    make start
    ```

    * API Firewallによって保護されたアプリケーション **httpbin**は、http://localhost:8080から利用できます。
    * API Firewallによって保護されていないアプリケーション **httpbin**は、http://localhost:8090から利用できます。デモデプロイメントをテストする場合、保護されていないアプリケーションにリクエストを送信して違いを確認できます。
4. デモのテストに進みます。## ステップ2: デモのテスト

以下のリクエストを使用して、デプロイ済みのAPI Firewallをテストすることができます:

* API Firewallが公開されていないパスに送信されたリクエストをブロックすることを確認します:

    ```bash
    curl -sD - http://localhost:8080/unexposed/path
    ```

    期待するレスポンス:

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 06:58:29 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```
* API Firewallが、整数データ型が必要なパラメータに文字列値が渡されたリクエストをブロックすることを確認します:

    ```bash
    curl -sD - http://localhost:8080/cache/arewfser
    ```

    期待するレスポンス:

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 06:58:29 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

    このケースは、API FirewallがアプリケーションをCache-Poisoned DoS攻撃から保護することを示しています。
* API Firewallが、以下の定義に合致しないクエリパラメータ`int`が必須であるリクエストをブロックすることを確認します:

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

    以下のリクエストを使用して定義をテストします:

    ```bash
    # 必須のクエリパラメータが欠けたリクエスト
    curl -sD - http://localhost:8080/get

    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:08 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0

    
    # intパラメータの値が適切な範囲内であるリクエスト
    curl -sD - http://localhost:8080/get?int=15

    # 期待するレスポンス
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 31 May 2021 07:09:38 GMT
    Content-Type: application/json
    Content-Length: 280
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true
    ...


    # intパラメータの値が範囲外であるリクエスト
    curl -sD - http://localhost:8080/get?int=5

    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:27 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # intパラメータの値が範囲外であるリクエスト
    curl -sD - http://localhost:8080/get?int=1000

    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:53 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # intパラメータの値が範囲外であるリクエスト
    # 潜在的な悪意: 8バイト整数のオーバーフローはスタックドロップとして応答できます
    curl -sD - http://localhost:8080/get?int=18446744073710000001

    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:04 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```
* API Firewallが、以下の定義に一致しないクエリパラメータ`str`を含むリクエストをブロックすることを確認します:

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

    以下のリクエストを使用して定義をテストします(`int`パラメータはまだ必要です):

    ```bash
    # strパラメータの値が定義済みの正規表現に一致しないリクエスト
    curl -sD - "http://localhost:8080/get?int=15&str=fasxxx.xxxawe-6354"

    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:42 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # strパラメータの値が定義済みの正規表現に一致しないリクエスト
    curl -sD - "http://localhost:8080/get?int=15&str=faswerffa-63sss54"
    
    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:42 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0


    # strパラメータの値が定義済みの正規表現に一致するリクエスト
    curl -sD - http://localhost:8080/get?int=15&str=ri0.2-3ur0-6354

    # 期待するレスポンス
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 31 May 2021 07:11:03 GMT
    Content-Type: application/json
    Content-Length: 331
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true
    ...


    # strパラメータの値が定義済みの正規表現に一致しないリクエスト
    # 潜在的な悪意: SQLインジェクション
    curl -sD - 'http://localhost:8080/get?int=15&str=";SELECT%20*%20FROM%20users.credentials;"'

    # 期待するレスポンス
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:12:04 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

## ステップ4: デモコードの停止

デモデプロイメントを停止し、環境をクリアするには、次のコマンドを使用します:

```bash
make stop
```
