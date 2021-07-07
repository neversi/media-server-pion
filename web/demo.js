let socket = new WebSocket("ws://84.201.153.156:8189/")
// let socket = new WebSocket("wss://janus.dar.tech/")
console.log("Attempting WebSocket connection")
socket.onopen = () => {
  console.log("Connected");
}

socket.onclose = (event) => {
  console.log("Socket Closed Connection: ", event);
}

socket.onerror = (error) => {
  console.log("Socket error: ", error);
}

let iceServers = [
  {
    urls: 'stun:stun.l.google.com:19302'
  }
]



let pcs = {
  "webcam": new RTCPeerConnection({
    iceServers: iceServers
  }),
  "screen": new RTCPeerConnection({
    iceServers: iceServers
  })
}
addTracks(pcs)

async function addTracks(pcs) {
  const media = await navigator.mediaDevices.getUserMedia(
                          {video: true, audio: true});
  const screen = await navigator.mediaDevices.getDisplayMedia(
                          {
                            video: {
                              cursor: "always"
                            },
                            audio: false
                          }
  )
  for (const track of media.getTracks()) {
    pcs["webcam"].addTrack(track);
  }
  for (const track of screen.getTracks()) {
    pcs["screen"].addTrack(track);
  }
  pcs["webcam"].createOffer().then(d => {
    pcs["webcam"].setLocalDescription(d)
  })
  pcs["screen"].createOffer().then(d => {
    pcs["screen"].setLocalDescription(d)
  })
}

for (const key in pcs) {
  pcs[key].oniceconnectionstatechange = e => console.log("[" + key + "] " + "ICE state: " + pcs[key].iceConnectionState)
  pcs[key].onicecandidate = event => {
    if (event.candidate === null) {
      request = {
        cmd: "connect",
        sdp: btoa(JSON.stringify(pcs[key].localDescription)),
        device: key
      }
      console.log(event)
      
      try {
        socket.send(JSON.stringify(request))
      } catch (e) {
        alert(e)
      }
    }
  }
}

socket.onmessage = message => {
  try {
    jsonResponse = JSON.parse(message.data)
    console.log(jsonResponse)
    pcs[jsonResponse.device].setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(jsonResponse.sdp))))
  } catch (e) {
    alert(e)
  }
}
