"""Original deterministic hand-painted texture construction for the Hewan props.
Run with Python 3 + Pillow + NumPy before Blender. No downloaded assets.
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter
import numpy as np, random, math
ROOT=Path(__file__).resolve().parent
OUT=ROOT/'textures'; OUT.mkdir(exist_ok=True)
random.seed(271828)
def pbr(name,im,rough=.85,strength=.8):
    im=im.convert('RGB'); im.save(OUT/f'{name}_albedo.png')
    a=np.asarray(im).astype(float)/255
    h=np.mean(a,axis=2)
    gy,gx=np.gradient(h); v=np.stack((-gx*strength,-gy*strength,np.ones_like(gx)),axis=-1);v/=np.linalg.norm(v,axis=-1)[...,None]
    Image.fromarray(np.uint8(np.clip((v*.5+.5)*255,0,255))).save(OUT/f'{name}_normal.png')
    r=np.clip(rough+(h-h.mean())*.14,0,1)
    Image.fromarray(np.uint8(r*255)).convert('RGB').save(OUT/f'{name}_roughness.png')
def wood(name,base,seed):
    rng=random.Random(seed); N=1024;y,x=np.mgrid[:N,:N]; nr=np.random.default_rng(seed)
    w=x+8*np.sin(y*.011)+3*np.sin(y*.039+x*.008)
    grain=5*np.sin(w*.18)+3*np.sin(w*.68)+2*np.sin(w*1.55)
    wash=4*np.sin(y*.008+x*.014)+nr.normal(0,1.2,(N,N))
    a=np.clip(np.array(base)[None,None,:]+(grain+wash)[...,None],0,255).astype('uint8')
    im=Image.fromarray(a);d=ImageDraw.Draw(im)
    for j in range(150):
        xx=rng.randrange(N);yy=rng.randrange(N);length=rng.randint(70,800);c=tuple(max(0,k-rng.randint(8,25)) for k in base)
        pts=[(xx+5*math.sin((yy+t)*.016+j)+2*math.sin((yy+t)*.039),yy+t) for t in range(0,length,8)]
        d.line(pts,fill=c,width=rng.choice([1,1,2,3]))
        if j%4==0:d.line([(a+2,b) for a,b in pts],fill=tuple(min(255,k+15) for k in base),width=1)
    for j in range(7):
        xx=rng.randrange(60,N-60);yy=rng.randrange(70,N-70)
        for rr in [38,27,16,7]:
            d.ellipse((xx-rr*.35,yy-rr,xx+rr*.35,yy+rr),outline=tuple(max(0,k-20) for k in base),width=2)
    pbr(name,im, .88, 2.2)
wood('wood',(105,82,57),17);wood('wood_light',(135,112,78),22);wood('bark',(91,83,63),31)
for name,base,seed in [('stone',(157,153,133),12),('metal',(65,75,70),15),('cloth',(77,111,109),16),('paper',(160,69,46),18),('rope',(151,129,88),19),('ceramic',(143,116,75),20)]:
    N=512;nr=np.random.default_rng(seed);y,x=np.mgrid[:N,:N]
    large=9*np.sin(x*.023+np.sin(y*.014))*np.sin(y*.029)+3*np.sin((x+y)*.087)
    fine=nr.normal(0,1.7,(N,N))
    if name=='cloth':fine+=(np.sin(x*np.pi)+np.sin(y*np.pi))*.8
    if name=='paper':large+=8*np.sin(x*.11)+4*np.sin(x*.42)
    if name=='rope':large+=8*np.sin((x+y*.7)*.2)
    im=Image.fromarray(np.uint8(np.clip(np.array(base)+(large+fine)[...,None],0,255)))
    if name=='stone':
        d=ImageDraw.Draw(im)
        for i in range(220):
            a=random.randrange(N);b=random.randrange(N);r=random.choice([1,1,2,3,5]);d.ellipse((a,b,a+r*2,b+r),fill=(143+random.randrange(25),140+random.randrange(20),120+random.randrange(22)))
    pbr(name,im,.94 if name not in ['metal','ceramic'] else .6,1.8)
# Four dense, asymmetrical broad-leaf masses. Pointed painted leaves overlap into
# small branching crowns; soft holes and scalloped alpha edges avoid flat card silhouettes.
for variant in range(4):
    rng=random.Random(400+variant);N=512;im=Image.new('RGBA',(N,N),(0,0,0,0));d=ImageDraw.Draw(im)
    positions=[]
    for j in range(144):
        ang=j*2.399963+variant;rr=math.sqrt((j+.5)/144)
        cx=256+math.cos(ang)*rr*(190+18*math.sin(ang*3+variant))
        cy=256+math.sin(ang)*rr*(178+22*math.cos(ang*2-variant))
        positions.append((cy,cx,rr,ang))
    # Back-to-front ordering creates clear overlapping brush-stroke shapes.
    for cy,cx,rr,theta in sorted(positions):
        ang=theta+rng.uniform(-1.2,1.2);length=rng.uniform(58,86);width=length*rng.uniform(.38,.56)
        dx=math.cos(ang);dy=math.sin(ang);pts=[]
        for side,ks in [(1,range(17)),(-1,reversed(range(17)))]:
            for k in ks:
                tt=k/16;long=(tt-.5)*length;wide=math.sin(tt*math.pi)*width*.5*side
                pts.append((cx+dx*long-dy*wide,cy+dy*long+dx*wide))
        shade=rng.randint(-10,14);sun=int((1-rr)*8)
        col=(57+shade+sun+variant*2,83+shade+sun,49+shade//2+sun,255)
        d.polygon(pts,fill=col)
        d.line(pts[:17],fill=(47+shade,69+shade,41+shade//2,255),width=2)
        d.line([(cx-dx*length*.40,cy-dy*length*.40),(cx+dx*length*.40,cy+dy*length*.40)],fill=(82+shade+sun,106+shade+sun,60+shade//2,255),width=2)
        # Broad, off-center painted highlight instead of a photographic normal map.
        d.line([(cx-dx*length*.15-dy*width*.13,cy-dy*length*.15+dx*width*.13),(cx+dx*length*.25-dy*width*.12,cy+dy*length*.25+dx*width*.12)],fill=(70+shade+sun,96+shade+sun,54+shade//2,255),width=3)
    im.save(OUT/f'leaves_{variant}_albedo.png')
    solid=Image.new('RGB',(N,N),(49+variant*2,72+variant*2,43));sd=ImageDraw.Draw(solid)
    # Fully tiled surface painting for volume UVs: no empty border or flat green caps.
    for j in range(260):
        cx=rng.randrange(N);cy=rng.randrange(N);ang=rng.uniform(0,math.tau);length=rng.uniform(60,100);width=length*rng.uniform(.4,.6);dx=math.cos(ang);dy=math.sin(ang);shade=rng.randint(-11,19)
        for ox in [-N,0,N]:
            for oy in [-N,0,N]:
                pts=[]
                for side,ks in [(1,range(13)),(-1,reversed(range(13)))]:
                    for k in ks:
                        tt=k/12;ll=(tt-.5)*length;ww=math.sin(tt*math.pi)*width*.5*side;pts.append((cx+ox+dx*ll-dy*ww,cy+oy+dy*ll+dx*ww))
                sd.polygon(pts,fill=(62+shade,87+shade,49+shade//2))
                sd.line(pts[:13],fill=(46+shade,68+shade,39+shade//2),width=2)
                sd.line([(cx+ox-dx*length*.38,cy+oy-dy*length*.38),(cx+ox+dx*length*.38,cy+oy+dy*length*.38)],fill=(83+shade,107+shade,59+shade//2),width=2)
    pbr(f'leaf_mass_{variant}',solid,.97,.6)
print('Original PNG textures generated:',len(list(OUT.glob('*.png'))))
