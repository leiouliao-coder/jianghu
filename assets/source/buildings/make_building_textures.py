"""Original deterministic brush-painted PBR textures. No third-party resources.
Run with system Python (requires numpy and Pillow) before build_buildings.py.
"""
from PIL import Image, ImageDraw, ImageFilter
import numpy as np, math, random
from pathlib import Path
ROOT=Path(__file__).resolve().parent/'textures'; ROOT.mkdir(exist_ok=True)
N=512
palettes={
'plaster':((216,208,182),.94,0),'wood':((103,76,49),.82,0),'roof':((76,91,95),.86,0),
'roof_light':((86,101,104),.86,0),'roof_dark':((65,79,83),.86,0),'roof_warm':((83,91,87),.86,0),
'stone':((120,122,111),.95,0),'iron':((63,67,65),.5,.8),'brass':((137,108,54),.47,.65),
'cloth':((123,47,33),.95,0),'coal':((34,32,30),1,0),'ember':((240,98,16),.75,0),
'paper':((184,175,151),.96,0)}
rng=np.random.default_rng(1049)
for kind,(color,rough,metal) in palettes.items():
    low=rng.normal(0,1,(32,32)); noise=np.asarray(Image.fromarray(((low-low.min())/(low.max()-low.min())*255).astype('uint8')).resize((N,N),Image.Resampling.BICUBIC)).astype(float)/255-.5
    y,x=np.mgrid[0:N,0:N]; h=.5+.11*noise
    a=np.zeros((N,N,3),float)+np.array(color); a+=noise[:,:,None]*28
    if kind=='wood':
        grain=np.sin(x*.19+np.sin(y*.013)*3+np.sin(y*.044+x*.013)*1.5)
        grain+=.45*np.sin(x*.75+np.sin(y*.027)*5)
        a+=grain[:,:,None]*8; h+=grain*.06
        # Pale longitudinal brush glazes and darker growth streaks.
        for j in range(22):
            cx=int(rng.uniform(0,N)); w=rng.uniform(1,5); f=np.exp(-((x-cx+4*np.sin(y*.018+j))/w)**2)
            a+=f[:,:,None]*rng.uniform(-16,12); h+=f*.025
        for cx,cy in [(145,170),(372,390)]:
            d=((x-cx)/29)**2+((y-cy)/61)**2
            rings=np.sin(np.sqrt(d)*12)*np.exp(-d*.24)
            a-=rings[:,:,None]*10; h+=rings*.025
    elif kind.startswith('roof'):
        # Each mapped patch is a whole dark-blue fired tile, warm rubbed edges.
        edge=np.exp(-((x-8)/11)**2)+np.exp(-((x-504)/11)**2)
        edge2=np.exp(-((y-7)/9)**2)+np.exp(-((y-505)/10)**2)
        a+=edge[:,:,None]*17+edge2[:,:,None]*13
        brush=np.sin(x*.07+np.sin(y*.027))*3+np.sin(x*.23+y*.007)*1.5
        a+=brush[:,:,None]; h+=np.sin(x/N*math.pi)*.18+edge2*.04
        a[:10]*=.55; a[10:19]+=22; a[-8:]*=.65
        seam=np.exp(-((y-256)/5)**2); lip=np.exp(-((y-265)/7)**2); a-=seam[:,:,None]*24; a+=lip[:,:,None]*12; h-=seam*.035
    elif kind=='plaster':
        # Hand-smoothed plaster clouds and fine irregular hairline crazing.
        a+=np.sin(y*.018+x*.006)[:,:,None]*3
        for j in range(8):
            xx=int(rng.uniform(0,N)); yy=int(rng.uniform(0,N)); length=int(rng.uniform(12,100))
            for k in range(length):
                px=(xx+int(2*math.sin(k*.14)+k*.12))%N; py=(yy+k)%N
                a[py,px]-=14; h[py,px]-=.012
    elif kind=='stone':
        # Block tile with chipped pale rim; UV repetition establishes masonry joints.
        dist=np.minimum.reduce([x,y,N-1-x,N-1-y]); rim=dist<10; inner=(dist>=10)&(dist<18)
        a[rim]*=.55; a[inner]+=17; h[rim]-=.2; h[inner]+=.02
        a+=np.sin(x*.035+y*.017)[:,:,None]*3
    elif kind in ['iron','brass']:
        a+=np.sin(y*.24)[:,:,None]*2; h+=rng.normal(0,.006,(N,N))
    elif kind=='cloth':
        weave=(np.sin(x*math.pi/2)+np.sin(y*math.pi/2))
        a+=weave[:,:,None]*3; h+=weave*.014
    elif kind=='coal':
        a+=noise[:,:,None]*25; h+=noise*.2
    elif kind=='ember':
        hot=(np.sin(x*.029)*np.sin(y*.047)+noise*2)
        a[:,:,0]+=hot*10; a[:,:,1]+=hot*35; a[:,:,2]+=hot*8; h+=noise*.2
    a=np.clip(a,0,255).astype('uint8'); Image.fromarray(a).save(ROOT/(kind+'_basecolor.png'))
    gy,gx=np.gradient(h); normal=np.stack([-gx*12,-gy*12,np.ones((N,N))],-1); normal/=np.linalg.norm(normal,axis=-1)[:,:,None]
    Image.fromarray(np.clip((normal*.5+.5)*255,0,255).astype('uint8')).save(ROOT/(kind+'_normal.png'))
    orm=np.zeros((N,N,3)); orm[:,:,0]=255; orm[:,:,1]=np.clip(rough*255+noise*12,0,255); orm[:,:,2]=metal*255
    Image.fromarray(orm.astype('uint8')).save(ROOT/(kind+'_orm.png'))
print('Wrote original 512px PNG PBR sets:',', '.join(palettes))
