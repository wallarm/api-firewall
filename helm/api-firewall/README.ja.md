# Wallarm API FirewallのためのHelmチャート

このチャートは、[Helm]（https://helm.sh/）パッケージマネージャを使用して[Kubernetes]（http://kubernetes.io/）クラスタ上にWallarm API Firewallのデプロイメントを初期化します。

このチャートはまだ公開のHelmレジストリにはアップロードされていません。Helmチャートのデプロイのためには、このリポジトリを使用してください。

## 必要条件

* Kubernetes 1.16 又はそれ以降
* Helm 2.16 又はそれ以降

## デプロイメント

Wallarm API Firewall Helmチャートをデプロイするには：

1. まだ追加していない場合は、リポジトリを追加してください：

```bash
helm repo add wallarm https://charts.wallarm.com
```

2. helmチャートの最新バージョンを取得します：

```bash
helm fetch wallarm/api-firewall
tar -xf api-firewall*.tgz
```

3. コードコメントに従って、`api-firewall/values.yaml` ファイルを変更してチャートを設定します。

4. このHelmチャートからWallarm API Firewallをデプロイします。

このHelmチャートのデプロイメントの例を確認したい場合は、私たちの[Kuberentesデモ]（https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes）を走らせることができます。