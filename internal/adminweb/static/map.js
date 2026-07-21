window.DBAMap=(()=>{
  const state={maps:[],map:null,points:[],selected:"",layer:"topographic",zoom:3,centerX:0,centerZ:0,ready:false,images:new Map()};
  const $=id=>document.getElementById(id),canvas=$("map-canvas"),ctx=canvas.getContext("2d");

  async function init(){
    try{
      const response=await fetch("/v1/map/maps");
      if(!response.ok)return;
      const data=await response.json();
      state.maps=data.maps||[];state.defaultMap=data.default_map||"";
      if(!state.maps.length)return;
      const select=$("map-select");
      select.replaceChildren(...state.maps.map(map=>new Option(map.name.replaceAll("_"," "),map.name)));
      select.addEventListener("change",()=>chooseMap(select.value,true));
      $("map-layer").addEventListener("change",event=>{state.layer=event.target.value;state.images.clear();render()});
      $("map-fit").addEventListener("click",fit);
      $("map-in").addEventListener("click",()=>zoomBy(1));
      $("map-out").addEventListener("click",()=>zoomBy(-1));
      new ResizeObserver(render).observe(canvas);
      state.ready=true;
    }catch(_){state.ready=false}
  }

  function setTimeline(timeline){
    state.points=(timeline.entries||[]).filter(entry=>entry.location).map(entry=>({...entry.location,time:entry.server_time_ms,authority:entry.authority,eventType:entry.event_type}));
    state.selected=timeline.session.player_session_id;
    const wanted=(timeline.map_id||state.defaultMap||"").toLowerCase();
    if(!state.ready)return;
    $("map-panel").classList.remove("hidden");
    const found=wanted?state.maps.find(map=>map.name.toLowerCase()===wanted):state.maps[0];
    if(!found){state.map=null;$("map-status").textContent=`Map ${wanted||"unknown"} is not available from DZMap`;return}
    chooseMap(found.name,false);
    $("map-status").textContent=state.points.length?`${state.points.length} located events · ${found.name}`:"No coordinates captured in this session";
    fit();
  }

  function chooseMap(name,refit){
    state.map=state.maps.find(map=>map.name===name)||state.maps[0];
    if(!state.map)return;
    $("map-select").value=state.map.name;
    const layer=$("map-layer");
    for(const option of layer.options)option.disabled=option.value==="topographic"?!!state.map.no_topographic:!!state.map.no_satellite;
    if((state.layer==="topographic"&&state.map.no_topographic)||(state.layer==="satellite"&&state.map.no_satellite)){
      state.layer=state.map.no_topographic?"satellite":"topographic";layer.value=state.layer;
    }
    $("map-attribution").textContent=state.map.attribution||"Map tiles supplied by the configured DZMap instance";
    state.images.clear();
    if(refit)fit();else render();
  }

  function fit(){
    if(!state.map)return;
    if(!state.points.length){state.centerX=state.map.size/2;state.centerZ=state.map.size/2;state.zoom=Math.min(2,state.map.zoom||6);render();return}
    const xs=state.points.map(point=>point.x),zs=state.points.map(point=>point.z),minX=Math.min(...xs),maxX=Math.max(...xs),minZ=Math.min(...zs),maxZ=Math.max(...zs);
    state.centerX=(minX+maxX)/2;state.centerZ=(minZ+maxZ)/2;
    const width=Math.max(1,canvas.clientWidth-100),height=Math.max(1,canvas.clientHeight-90),spanX=Math.max(600,maxX-minX+300),spanZ=Math.max(600,maxZ-minZ+300);
    state.zoom=0;
    for(let zoom=0;zoom<=Math.min(20,state.map.zoom||6);zoom++){
      const scale=256*(2**zoom)/state.map.size;
      if(spanX*scale<=width&&spanZ*scale<=height)state.zoom=zoom;else break;
    }
    render();
  }

  function zoomBy(delta){if(!state.map)return;state.zoom=Math.max(0,Math.min(state.map.zoom||6,state.zoom+delta));render()}
  function focus(location){if(!state.map||!location)return;state.centerX=location.x;state.centerZ=location.z;state.zoom=Math.max(state.zoom,Math.min(5,state.map.zoom||6));render()}

  function render(){
    if(!state.ready||!state.map||!canvas.clientWidth)return;
    const dpr=window.devicePixelRatio||1,width=canvas.clientWidth,height=canvas.clientHeight;
    if(canvas.width!==Math.round(width*dpr)||canvas.height!==Math.round(height*dpr)){canvas.width=Math.round(width*dpr);canvas.height=Math.round(height*dpr)}
    ctx.setTransform(dpr,0,0,dpr,0,0);ctx.fillStyle="#0c100e";ctx.fillRect(0,0,width,height);
    const world=256*(2**state.zoom),scale=world/state.map.size,centerPixelX=state.centerX*scale,centerPixelY=(state.map.size-state.centerZ)*scale,left=centerPixelX-width/2,top=centerPixelY-height/2;
    const minTileX=Math.max(0,Math.floor(left/256)),maxTileX=Math.min(2**state.zoom-1,Math.floor((left+width)/256)),minTileY=Math.max(0,Math.floor(top/256)),maxTileY=Math.min(2**state.zoom-1,Math.floor((top+height)/256));
    for(let tileY=minTileY;tileY<=maxTileY;tileY++)for(let tileX=minTileX;tileX<=maxTileX;tileX++){
      const drawX=tileX*256-left,drawY=tileY*256-top,key=`${state.map.name}/${state.layer}/${state.zoom}/${tileX}/${tileY}`,image=state.images.get(key);
      if(image?.complete&&image.naturalWidth)ctx.drawImage(image,drawX,drawY,256,256);
      else if(!image){const next=new Image;next.onload=()=>requestAnimationFrame(render);next.src=`/v1/map/tiles/${encodeURIComponent(state.map.name)}/${state.layer}/${state.zoom}/${tileX}/${tileY}.webp`;state.images.set(key,next)}
    }
    drawTracks(width,height,scale,left,top);
  }

  function drawTracks(width,height,scale,left,top){
    const groups=new Map;
    for(const point of state.points){const subject=point.subject_session_id||"unknown";if(!groups.has(subject))groups.set(subject,[]);groups.get(subject).push(point)}
    for(const [subject,points] of groups){
      points.sort((a,b)=>a.time-b.time);const primary=subject===state.selected;
      ctx.strokeStyle=primary?"#b8f36b":"#83b8ef";ctx.lineWidth=primary?2.5:1.5;ctx.lineJoin="round";ctx.lineCap="round";ctx.beginPath();
      points.forEach((point,index)=>{const pixel=screen(point,scale,left,top);index?ctx.lineTo(pixel.x,pixel.y):ctx.moveTo(pixel.x,pixel.y)});ctx.stroke();
      for(const point of points){
        const pixel=screen(point,scale,left,top);if(pixel.x<0||pixel.y<0||pixel.x>width||pixel.y>height)continue;
        if(point.accuracy_metres>10){ctx.beginPath();ctx.arc(pixel.x,pixel.y,Math.max(6,point.accuracy_metres*scale),0,Math.PI*2);ctx.fillStyle="#f0c86822";ctx.fill();ctx.strokeStyle="#f0c868aa";ctx.lineWidth=1;ctx.stroke()}
        ctx.beginPath();ctx.arc(pixel.x,pixel.y,point.authority==="B"?4:5,0,Math.PI*2);ctx.fillStyle=point.authority==="B"?"#f0c868":primary?"#b8f36b":"#83b8ef";ctx.fill();ctx.strokeStyle="#101412";ctx.lineWidth=2;ctx.stroke();
      }
    }
  }

  function screen(point,scale,left,top){return{x:point.x*scale-left,y:(state.map.size-point.z)*scale-top}}
  return{init,setTimeline,focus};
})();
