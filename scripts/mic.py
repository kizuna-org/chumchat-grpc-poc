import pyaudio
import sys

def main():
    chunk = 1024  # 読み取りチャンクサイズ (Goのコードに合わせて調整可能)
    FORMAT = pyaudio.paInt16  # Goのコードに合わせて LINEAR16 (16bit integer)
    CHANNELS = 1  # モノラル
    RATE = 16000  # Goのコードに合わせて 16000Hz

    p = pyaudio.PyAudio()

    stream = p.open(format=FORMAT,
                    channels=CHANNELS,
                    rate=RATE,
                    input=True,
                    frames_per_buffer=chunk)

    print("* recording")

    try:
        while True:
            data = stream.read(chunk)
            sys.stdout.buffer.write(data)  # バイナリデータを標準出力に書き込む
    except KeyboardInterrupt:
        print("* done recording")
    finally:
        stream.stop_stream()
        stream.close()
        p.terminate()

if __name__ == "__main__":
    main()
