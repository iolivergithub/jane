To install ollama and run this:

```sh
curl -fsSL https://ollama.com/install.sh | sh
ollama pull qwen2.5:7b  
ollama run qwen2.5:7b "hello"
curl http://localhost:11434

#assuming we are in /home/ian/jane/aijane/venv
sudo apt install python3-pip
python3 -m venv /home/ian/jane/aijane/venv
/home/ian/jane/aijane/venv/bin/pip3 install ollama paho-mqtt
/home/ian/jane/aijane/venv/bin/python3 jane_situation.py mqtt --broker 127.0.0.1
```

NB: port 1883 is assumed
