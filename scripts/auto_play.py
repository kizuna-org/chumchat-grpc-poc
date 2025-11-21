import os
import time
import subprocess

def play_mp3(filepath):
    """MP3ファイルを再生する関数 (プラットフォーム依存)"""
    if os.name == 'nt':  # Windows
        try:
            os.startfile(filepath)  # デフォルトの関連付けられたプログラムで開く
        except OSError:
            print(f"Windows Media Player など、MP3再生可能なプログラムが関連付けられていない可能性があります。: {filepath}")
    elif os.name == 'posix':  # macOS, Linux
        try:
            subprocess.run(['afplay', filepath], check=True) # macOS (afplay)
        except FileNotFoundError:
            try:
                subprocess.run(['mpg123', filepath], check=True) # Linux (mpg123)
            except FileNotFoundError:
                try:
                    subprocess.run(['play', filepath], check=True) # Linux (sox)
                except FileNotFoundError:
                    print(f"MP3再生可能なコマンドが見つかりません (afplay, mpg123, play のいずれかが必要です): {filepath}")
            except subprocess.CalledProcessError:
                print(f"mpg123 での再生中にエラーが発生しました: {filepath}")
        except subprocess.CalledProcessError:
            print(f"afplay での再生中にエラーが発生しました: {filepath}")
    else:
        print(f"お使いのOS ({os.name}) では自動再生はサポートされていません。")

filepath = "./output.mp3"
last_modified_time = 0

print(f"{filepath} の更新を監視します。Ctrl+C で停止します。")

try:
    while True:
        if os.path.exists(filepath):
            current_modified_time = os.path.getmtime(filepath)
            if current_modified_time > last_modified_time:
                if last_modified_time != 0: # 初回起動時は再生しないようにする
                    print(f"{filepath} が更新されました。再生します...")
                    play_mp3(filepath)
                last_modified_time = current_modified_time
        else:
            if last_modified_time != 0: # ファイルが削除された場合
                print(f"{filepath} が削除されました。監視を継続します...")
                last_modified_time = 0 # ファイルが再作成されたときに正しく検出するため

        time.sleep(1) # 1秒ごとに更新を確認

except KeyboardInterrupt:
    print("\n監視を終了しました。")
