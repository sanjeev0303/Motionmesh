import wave
import struct

with wave.open('dummy.wav', 'w') as f:
    f.setnchannels(1)
    f.setsampwidth(2)
    f.setframerate(44100)
    for _ in range(44100):
        f.writeframesraw(struct.pack('<h', 0))
