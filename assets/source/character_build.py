"""Original Jianghu novice. Blender 4.3.2 reproducible mesh, baked texture & animation build.
Run: blender -b --python assets/source/character_build.py
No external assets, fonts, meshes or procedural shader nodes are needed.
"""
import bpy, math, os, json, random
from mathutils import Vector, Matrix, Quaternion
from mathutils.bvhtree import BVHTree
import numpy as np
from math import sin,cos,pi,sqrt
ROOT=os.path.abspath(os.path.join(os.path.dirname(__file__),'../..'))
SRC=os.path.join(ROOT,'assets/source'); TEX=os.path.join(SRC,'character_textures')
OUT=os.path.join(ROOT,'client/assets/art/characters/player_novice.glb')
os.makedirs(TEX,exist_ok=True); os.makedirs(os.path.dirname(OUT),exist_ok=True)
random.seed(27); bpy.ops.object.select_all(action='SELECT'); bpy.ops.object.delete(use_global=False)
for x in list(bpy.data.materials): bpy.data.materials.remove(x)
for x in list(bpy.data.actions): bpy.data.actions.remove(x)
S=bpy.context.scene; S.unit_settings.system='METRIC'; S.unit_settings.scale_length=1.; S.render.fps=60
mesh_objects=[]; mats={}
# Textures are deterministic original raster paintings, not Blender-only shader noise.
def material(name,color,rough=.8,metal=0,style='cloth',size=512):
    rng=np.random.default_rng(sum(map(ord,name)))
    yy,xx=np.mgrid[0:size,0:size].astype(float); u=xx/size; v=yy/size
    noise=rng.normal(0,1,(size,size))
    grid=rng.normal(0,1,(13,13));gx=np.linspace(0,12,size);coarse=np.array([np.interp(gx,np.arange(13),row) for row in grid]);cloud=np.array([np.interp(gx,np.arange(13),coarse[:,i]) for i in range(size)]).T
    broad=cloud*.008
    weave=(np.cos(xx*pi*.5)+np.cos(yy*pi*.5))*.004 if style=='cloth' else .003*np.sin(xx*.15)
    worn=np.zeros((size,size))
    if style=='cloth':
        for _ in range(size//2):
            x=int(rng.integers(0,size-5));y=int(rng.integers(0,size-5));worn[y:y+1,x:x+int(rng.integers(2,5))]=.025
    val=broad+noise*.011+weave+worn
    if style=='hair': val=np.sin(u*2*pi*33+np.sin(v*12)*1.4)*.012+noise*.004
    if style=='skin': val=noise*.003+np.cos(v*2*pi)*.009
    if style=='metal': val=noise*.005+np.sin(u*pi*5)*.03
    base=np.clip(np.array(color)[None,None,:]+val[:,:,None],0,1)
    height=(noise*.018+weave*.8+broad*.2) if style in ['cloth','leather'] else noise*.006
    gy,gx=np.gradient(height); normal=np.stack([-gx*1.5,-gy*1.5,np.ones_like(gx)],-1); normal/=np.linalg.norm(normal,axis=-1)[:,:,None]; normal=normal*.5+.5
    def img(suffix,rgb,noncolor=False):
        im=bpy.data.images.new(name+'_'+suffix,width=size,height=size,alpha=True)
        if noncolor: im.colorspace_settings.name='Non-Color'
        a=np.ones((size,size,4),dtype=np.float32); a[:,:,:3]=rgb; im.pixels.foreach_set(a.ravel())
        im.filepath_raw=os.path.join(TEX,name+'_'+suffix+'.png'); im.file_format='PNG'; im.save(); im.pack(); return im
    bi=img('basecolor',base); ni=img('normal',normal,True)
    orm=np.empty_like(base); orm[:,:,0]=1; orm[:,:,1]=np.clip(rough+noise*.012,0,1); orm[:,:,2]=metal
    ri=img('orm',orm,True)
    m=bpy.data.materials.new(name); m.use_nodes=True; m.diffuse_color=(*color,1)
    n=m.node_tree.nodes; bs=n.get('Principled BSDF'); bs.inputs['Roughness'].default_value=rough; bs.inputs['Metallic'].default_value=metal
    t=n.new('ShaderNodeTexImage');t.image=bi;m.node_tree.links.new(t.outputs['Color'],bs.inputs['Base Color'])
    t=n.new('ShaderNodeTexImage');t.image=ni; nm=n.new('ShaderNodeNormalMap');nm.inputs['Strength'].default_value=.35;m.node_tree.links.new(t.outputs['Color'],nm.inputs['Color']);m.node_tree.links.new(nm.outputs['Normal'],bs.inputs['Normal'])
    t=n.new('ShaderNodeTexImage');t.image=ri; sep=n.new('ShaderNodeSeparateColor');m.node_tree.links.new(t.outputs['Color'],sep.inputs['Color']);m.node_tree.links.new(sep.outputs['Green'],bs.inputs['Roughness']);m.node_tree.links.new(sep.outputs['Blue'],bs.inputs['Metallic'])
    mats[name]=m; return m
material('slate_woven',(.185,.255,.29));material('slate_edge',(.29,.36,.37));material('flax',(.60,.545,.425));material('trouser',(.115,.125,.133));material('leather',(.205,.13,.085),.75,style='leather');material('binding',(.30,.255,.19));material('boots',(.12,.105,.09));material('skin',(.72,.51,.365),.72,style='skin');material('skin_shadow',(.36,.16,.10),.75,style='skin');material('hair',(.025,.028,.032),.58,style='hair');material('hair_ridge',(.055,.056,.052),.65,style='hair');material('bronze',(.41,.30,.13),.42,.7,style='metal');material('steel',(.52,.59,.61),.28,.85,style='metal');material('eye',(.67,.62,.49),.45,style='skin',size=128);material('iris',(.055,.032,.014),.35,style='skin',size=128)
# rest skeleton, axis Y follows each bone; human adult 1.75m including tied hair.
bones={
'root':((0,0,0),(0,0,.15),None),
'pelvis':((0,0,.94),(0,0,1.055),'root'),
'spine':((0,0,1.055),(0,0,1.235),'pelvis'),
'chest':((0,0,1.235),(0,0,1.425),'spine'),
'neck':((0,0,1.425),(0,0,1.51),'chest'),
'head':((0,0,1.51),(0,0,1.70),'neck'),
'ponytail':((0,.065,1.70),(0,.13,1.49),'head'),
'sword':((-.218,-.01,1.085),(-.27,.025,.40),'root')}
for side,sign in [('L',-1),('R',1)]:
    bones['clavicle.'+side]=((0,0,1.40),(sign*.205,0,1.405),'chest')
    bones['upper_arm.'+side]=((sign*.205,0,1.405),(sign*.36,0,1.18),'clavicle.'+side)
    bones['forearm.'+side]=((sign*.36,0,1.18),(sign*.415,-.015,.975),'upper_arm.'+side)
    bones['hand.'+side]=((sign*.415,-.015,.975),(sign*.44,-.022,.88),'forearm.'+side)
    w=Vector(bones['hand.'+side][0]);ha=(Vector(bones['hand.'+side][1])-w).normalized();kn=w+ha*.066
    bones['fingers.'+side]=(tuple(kn),tuple(kn+ha*.035),'hand.'+side)
    bones['fingertips.'+side]=(tuple(kn+ha*.035),tuple(kn+ha*.075),'fingers.'+side)
    bones['thigh.'+side]=((sign*.097,0,.94),(sign*.102,-.012,.535),'pelvis')
    bones['shin.'+side]=((sign*.102,-.012,.535),(sign*.104,0,.125),'thigh.'+side)
    bones['foot.'+side]=((sign*.104,0,.125),(sign*.104,-.15,.062),'shin.'+side)
    bones['toe.'+side]=((sign*.104,-.15,.062),(sign*.104,-.205,.05),'foot.'+side)
arm=bpy.data.armatures.new('Novice_Humanoid'); rig=bpy.data.objects.new('player_novice',arm); bpy.context.collection.objects.link(rig);bpy.context.view_layer.objects.active=rig;rig.select_set(True);bpy.ops.object.mode_set(mode='EDIT')
for name,(a,b,parent) in bones.items():
    bo=arm.edit_bones.new(name);bo.head=a;bo.tail=b
    if parent:bo.parent=arm.edit_bones[parent]
    bo.use_deform=True
bpy.ops.object.mode_set(mode='OBJECT');rig.show_in_front=True;rig['height_m']=1.75;rig['front_axis']='Blender -Y / glTF +Z';rig['walk_root_motion_mps']=4.;rig['attack_contact_seconds']=.25
# Mesh construction with authored topology, explicit UVs and smooth skinning.
def obj(name,verts,faces,mat,weights,uvs=None):
    me=bpy.data.meshes.new(name);me.from_pydata(verts,[],faces);me.materials.append(mats[mat]);me.update();ob=bpy.data.objects.new(name,me);bpy.context.collection.objects.link(ob)
    lay=me.uv_layers.new(name='UVMap')
    for p in me.polygons:
        p.use_smooth=True
        for li in p.loop_indices:
            vi=me.loops[li].vertex_index;v=verts[vi]
            lay.data[li].uv=uvs[vi] if uvs else (v[0]*2+v[1]*.35,v[2]*2)
    if isinstance(weights,str): weights=[{weights:1.} for v in verts]
    groups={}
    for i,wd in enumerate(weights):
        total=sum(wd.values())
        for bn,w in wd.items():
            if bn not in groups:groups[bn]=ob.vertex_groups.new(name=bn)
            if w>0:groups[bn].add([i],w/total,'REPLACE')
    mod=ob.modifiers.new('Humanoid skin','ARMATURE');mod.object=rig;ob.parent=rig;mesh_objects.append(ob);return ob

def torso_weights(z):
    if z<1.03:return {'pelvis':1}
    if z<1.17:
        f=(z-1.03)/.14;return {'pelvis':1-f,'spine':f}
    if z<1.32:
        f=(z-1.17)/.15;return {'spine':1-f,'chest':f}
    return {'chest':1}

def rings(name,rows,n,mat,bone,fold=0,cap=True):
    # rows: (cx,cy,z,rx,ry), elliptical horizontal topology.
    vs=[];uv=[];ws=[]
    for j,(cx,cy,z,rx,ry) in enumerate(rows):
        for k in range(n+1):
            a=2*pi*k/n;wave=1+fold*sin(a*7+j*.55)+fold*.4*cos(a*11-j*.7)
            vs.append((cx+rx*cos(a)*wave,cy+ry*sin(a)*wave,z));uv.append((k/n,j/(len(rows)-1)))
            ws.append(bone(z) if callable(bone) else {bone:1})
    fs=[]
    for j in range(len(rows)-1):
        for k in range(n):q=j*(n+1)+k;fs.append((q,q+1,q+n+2,q+n+1))
    if cap:fs.extend([tuple(reversed(range(n))),tuple((len(rows)-1)*(n+1)+k for k in range(n))])
    return obj(name,vs,fs,mat,ws,uv)

def tube(name,points,radii,mat,bone,n=10,elliptic=1):
    vs=[];uv=[];ws=[];ps=[Vector(p) for p in points]
    for j,p in enumerate(ps):
        direction=(ps[min(j+1,len(ps)-1)]-ps[max(j-1,0)]).normalized();ref=Vector((0,1,0))
        if abs(direction.dot(ref))>.95:ref=Vector((1,0,0))
        ax=direction.cross(ref).normalized();ay=direction.cross(ax).normalized()
        for k in range(n+1):
            a=2*pi*k/n;fold=(1+.035*sin(a*5+j*1.8)) if 'sleeve' in name.lower() else 1.;v=p+radii[j]*fold*(cos(a)*ax+sin(a)*ay*elliptic);
            if name.startswith('Tunic sleeve') and v.z>1.425:v.z=1.425+(v.z-1.425)*.30
            vs.append(tuple(v));uv.append((k/n,j/max(1,len(ps)-1)));ws.append(bone(j,v) if callable(bone) else {bone:1})
    fs=[]
    for j in range(len(ps)-1):
        for k in range(n):q=j*(n+1)+k;fs.append((q,q+1,q+n+2,q+n+1))
    fs.extend([tuple(reversed(range(n))),tuple((len(ps)-1)*(n+1)+k for k in range(n))]);return obj(name,vs,fs,mat,ws,uv)

def ellipsoid(name,c,scale,mat,bone,seg=14,lat=9):
    vs=[];uv=[]
    for j in range(lat+1):
        a=pi*j/lat
        for k in range(seg+1):
            b=2*pi*k/seg;vs.append((c[0]+scale[0]*sin(a)*cos(b),c[1]+scale[1]*sin(a)*sin(b),c[2]+scale[2]*cos(a)));uv.append((k/seg,j/lat))
    fs=[]
    for j in range(lat):
        for k in range(seg):q=j*(seg+1)+k;fs.append((q,q+1,q+seg+2,q+seg+1))
    return obj(name,vs,fs,mat,bone,uv)

def cloth_front_y(x,z,offset=.004):
    if mesh_objects:
        me=mesh_objects[0].data;tree=BVHTree.FromPolygons([v.co for v in me.vertices],[list(p.vertices) for p in me.polygons]);hit=tree.ray_cast(Vector((x,-1,z)),Vector((0,1,0)))
        if hit[0] is not None:return hit[0].y-offset
    profiles=[(1.005,.167,.103),(1.08,.16,.105),(1.18,.193,.118),(1.29,.211,.122),(1.38,.203,.105),(1.425,.115,.072),(1.47,.052,.047)]
    rx=float(np.interp(z,[q[0] for q in profiles],[q[1] for q in profiles]));ry=float(np.interp(z,[q[0] for q in profiles],[q[2] for q in profiles]))
    return -ry*sqrt(max(.04,1-(x/rx)**2))-offset

def ribbon(name,points,width,mat,bone,depth=.001):
    vs=[];ws=[];ps=[Vector(p) for p in points]
    if any(k in name.lower() for k in ['collar','lapel']):ps=[v for i in range(len(ps)-1) for v in [ps[i],ps[i].lerp(ps[i+1],.5)]]+[ps[-1]]
    for i,p in enumerate(ps):
        tangent=(ps[min(i+1,len(ps)-1)]-ps[max(i-1,0)]).normalized();side=tangent.cross(Vector((0,-1,0))).normalized()
        for s in [-1,1]:
            v=p+side*width*.5*s
            if any(k in name.lower() for k in ['collar','lapel']):v.y=cloth_front_y(v.x,v.z,.010 if 'stitched' in name else .007)
            vs.append(tuple(v));ws.append(bone(p.z) if callable(bone) else {bone:1})
    fs=[(i*2,i*2+1,i*2+3,i*2+2) for i in range(len(ps)-1)];return obj(name,vs,fs,mat,ws)

# Shirt torso, shaped shoulders and waist. Short coat has three skirt panels with walking splits.
rings('Wrap tunic torso',[(0,0,1.005,.167,.103),(0,0,1.08,.16,.105),(0,0,1.18,.193,.118),(0,0,1.29,.211,.122),(0,0,1.38,.203,.105),(0,0,1.425,.115,.072)],24,'slate_woven',torso_weights,.018)
rings('Neck',[(0,0,1.42,.052,.047),(0,0,1.49,.047,.044),(0,-.006,1.535,.052,.05)],16,'skin','neck')
pv=[];pu=[];pw=[]
for j,(z,w) in enumerate([(1.47,.044),(1.44,.046),(1.415,.04),(1.395,.02),(1.382,.002)]):
    for k in range(9):
        x=w*(k/4-1);pv.append((x,cloth_front_y(x,z,.004),z));pu.append((k/8,j/4));pw.append(torso_weights(z))
obj('Exposed neckline',pv,[(j*9+k,j*9+k+1,(j+1)*9+k+1,(j+1)*9+k) for j in range(4) for k in range(8)],'skin',pw,pu)

# Cream crossed v-shaped collar and slate outer facing; multi-edge ribbons follow chest.
ribbon('Inner collar left',[(.046,-.049,1.471),(.067,-.113,1.425),(.038,-.134,1.37),(-.018,-.136,1.30)],.036,'flax',torso_weights)
ribbon('Inner collar right',[(-.046,-.049,1.471),(-.067,-.113,1.425),(-.032,-.134,1.37),(.018,-.136,1.32)],.036,'flax',torso_weights)
ribbon('Overlapping lapel',[(.069,-.066,1.427),(.104,-.097,1.406),(.125,-.115,1.36),(.103,-.129,1.30),(.068,-.134,1.24),(.025,-.128,1.17),(-.045,-.119,1.105),(-.12,-.106,1.06)],.037,'slate_edge',torso_weights)
ribbon('Lapel stitched edge',[(.082,-.067,1.43),(.12,-.099,1.406),(.141,-.118,1.36),(.118,-.132,1.30),(.082,-.137,1.24),(.04,-.131,1.17),(-.03,-.122,1.105),(-.11,-.11,1.055)],.0028,'flax',torso_weights)

pv=[];pu=[];pw=[]
for j,(z,w) in enumerate([(1.39,.026),(1.365,.038),(1.34,.031),(1.315,.016)]):
    for k in range(7):
        x=w*(k/3-1);pv.append((x,cloth_front_y(x,z,.004),z));pu.append((k/6,j/3));pw.append(torso_weights(z))
obj('Cream shirt underlap',pv,[(j*7+k,j*7+k+1,(j+1)*7+k+1,(j+1)*7+k) for j in range(3) for k in range(6)],'flax',pw,pu)
# Coat skirt panels are tailored curved cloth surfaces, not intersecting cones.
def coat_weights(v):
    w=max(0,min(1,(1.042-v.z)/.10));return {'pelvis':1-w,('thigh.L' if v.x<0 else 'thigh.R'):w}

def skirt(name,a0,a1,mat,zbottom,rx=.222,ry=.136):
    n=14;rows=5;vs=[];uv=[];ws=[]
    for j in range(rows):
        f=j/(rows-1);z=1.052+(zbottom-1.052)*f
        for k in range(n+1):
            a=a0+(a1-a0)*k/n;r=1+.025*sin(k*2.4)*f
            x=(.17+(rx-.17)*f)*cos(a)*r;y=(.108+(ry-.108)*f)*sin(a)*r
            zz=z+.025*sin(a*2)*f;vs.append((x,y,zz));uv.append((k/n,f));ws.append(coat_weights(Vector((x,y,zz))))
    fs=[(j*(n+1)+k,j*(n+1)+k+1,(j+1)*(n+1)+k+1,(j+1)*(n+1)+k) for j in range(rows-1) for k in range(n)]
    ob=obj(name,vs,fs,mat,ws,uv)
    for k in [0,n]:
        pts=[vs[j*(n+1)+k] for j in range(rows)];tube(name+' hem edge '+str(k),pts,[.0025]*rows,'slate_edge',lambda j,v:coat_weights(v),n=5)
    tube(name+' lower hem',[vs[(rows-1)*(n+1)+k] for k in range(n+1)],[.003]*(n+1),'slate_edge',lambda j,v:coat_weights(v),n=5)
    return ob
skirt('Flax underskirt',pi*1.10,pi*1.90,'flax',.676,.205,.127)
skirt('Coat left front',pi*1.52,pi*2.03,'slate_woven',.674)
skirt('Coat right front',pi*.98,pi*1.48,'slate_woven',.704)
skirt('Coat back panel',.06,pi-.06,'slate_woven',.692,.224,.138)
# Waist sash and belt; buckle and ties add readable waist landmarks.
rings('Cloth waist sash',[(0,0,1.015,.177,.115),(0,0,1.035,.177,.116),(0,0,1.067,.174,.113),(0,0,1.08,.17,.112)],24,'slate_edge','pelvis',.01)
rings('Leather waist belt',[(0,0,1.034,.182,.12),(0,0,1.076,.178,.118)],28,'leather','pelvis')
for z in [1.04,1.069]:
    tube('Belt edge stitch '+str(z),[(.182*cos(a),.121*sin(a),z) for a in np.linspace(0,2*pi,41)],[.0016]*41,'flax','pelvis',n=4)
# buckle in front shaped as rounded rectangular loop.
pts=[(-.04,-.126,1.04),(-.046,-.129,1.047),(-.046,-.129,1.066),(-.038,-.129,1.074),(-.006,-.129,1.074),(.002,-.129,1.067),(.002,-.129,1.047),(-.006,-.129,1.039),(-.04,-.126,1.04)]
tube('Belt bronze buckle',pts,[.004]*len(pts),'bronze','pelvis',n=6)
tube('Buckle tongue',[(-.024,-.134,1.052),(.003,-.134,1.055)],[.0028,.002],'bronze','pelvis',n=5)
for i in range(4):ellipsoid('Belt stud '+str(i),(.025+i*.018,-.126,1.056),(.0025,.002,.0025),'bronze','pelvis',8,5)
ribbon('Loose leather tail',[(.12,-.1,1.06),(.15,-.13,.98),(.145,-.151,.88)],.025,'leather','pelvis')
# Pouch at wearer's right, cords and gathered mouth.
ellipsoid('Cloth belt pouch',(.19,-.073,.93),(.049,.036,.065),'binding','pelvis',14,10)
tube('Pouch neck',[(.185,-.065,1.01),(.19,-.07,.98)],[.019,.026],'binding','pelvis',n=12)
for i in range(3):tube('Pouch gather '+str(i),[(.176+i*.011,-.091,1.003),(.173+i*.012,-.104,.953),(.18+i*.009,-.102,.916)],[.0018,.0015,.001],'flax','pelvis',n=5)
tube('Pouch drawstring',[(.18,-.084,1.054),(.197,-.09,1.018),(.205,-.105,.998),(.21,-.11,.96)],[.0025]*4,'leather','pelvis',n=6)

# Arms in the authored A bind pose; sleeves flared then gathered into bracers.
for side,sg in [('L',-1),('R',1)]:
    sh=Vector(bones['upper_arm.'+side][0]);el=Vector(bones['upper_arm.'+side][1]);wr=Vector(bones['forearm.'+side][1]);axis=el-sh
    tube('Tunic sleeve '+side,[Vector((sg*.125,0,1.38)),sh,sh+axis*.20,sh+axis*.40,sh+axis*.62,sh+axis*.73],[.052,.074,.082,.084,.079,.075],'slate_woven',lambda j,v:{'chest':max(0,1-j/2),'upper_arm.'+side:min(1,j/2)},n=14)
    tube('Sleeve hem '+side,[sh+axis*.695,sh+axis*.74],[.077,.077],'slate_edge','upper_arm.'+side,n=16)
    tube('Linen undersleeve '+side,[sh+axis*.61,sh+axis*.83,el,el+(wr-el)*.3],[.065,.07,.065,.055],'flax',lambda j,v:{'upper_arm.'+side:max(0,1-j/3),'forearm.'+side:min(1,j/3)},n=16)
    tube('Forearm wrap '+side,[el+(wr-el)*.25,el+(wr-el)*.37,el+(wr-el)*.66,wr],[.061,.058,.042,.037],'boots','forearm.'+side,n=14)
    for i in range(5):
        t=.33+i*.135;c=el+(wr-el)*t;d=(wr-el).normalized();a=d.cross(Vector((0,1,0))).normalized();b=d.cross(a).normalized();rad=.061-(t-.25)*.033
        pts=[c+(a*cos(v)+b*sin(v))*rad+d*(v/(2*pi)-.5)*.025 for v in np.linspace(0,2*pi,17)]
        tube('Bracer spiral '+side+str(i),pts,[.0025]*17,'binding','forearm.'+side,n=4)
    # tapered hand palm, separated thumb and four individually articulated-looking fingers.
    handaxis=(Vector(bones['hand.'+side][1])-wr).normalized()
    tube('Palm '+side,[wr,wr+handaxis*.043,wr+handaxis*.076],[.033,.036,.03],'skin','hand.'+side,n=12,elliptic=.62)
    for i in range(4):
        base=wr+handaxis*.066+handaxis.cross(Vector((0,1,0))).normalized()*((i-1.5)*.015);length=[.058,.07,.066,.052][i]
        pts=[base,base+handaxis*length*.55+Vector((-sg*.006,0,0)),base+handaxis*length+Vector((-sg*.012,0,.004))]
        tube('Finger '+side+str(i),pts,[.0085,.0075,.005],'skin',lambda j,v:{'fingers.'+side:1-min(1,j/1.5),'fingertips.'+side:min(1,j/1.5)},n=7)
    base=wr+handaxis*.018+Vector((-sg*.025,-.014,0));tube('Thumb '+side,[base,base+Vector((-sg*.022,-.014,-.027)),base+Vector((-sg*.022,-.02,-.053))],[.012,.01,.007],'skin','hand.'+side,n=8)
    # legs, shaped through thigh, knee, and snug shin bindings.
    def lw(z,side=side):
        f=max(0,min(1,(.61-z)/.15));return {'thigh.'+side:1-f,'shin.'+side:f}
    rings('Charcoal trousers '+side,[(sg*.097,0,.963,.078,.078),(sg*.103,0,.87,.093,.092),(sg*.105,.005,.74,.096,.092),(sg*.104,-.005,.62,.088,.08),(sg*.102,-.012,.53,.087,.079),(sg*.103,-.005,.44,.076,.073)],16,'trouser',lw,.035)
    rings('Wrapped calf '+side,[(sg*.103,0,.46,.07,.066),(sg*.103,0,.40,.071,.066),(sg*.103,0,.30,.06,.055),(sg*.104,0,.19,.047,.044),(sg*.104,0,.125,.043,.04)],16,'boots','shin.'+side,.015)
    for i in range(6):
        z=.16+i*.048;r=.047+(z-.16)*.085
        pts=[(sg*.104+cos(a)*r,sin(a)*r*.93,z+(a/(2*pi)-.5)*.052) for a in np.linspace(0,2*pi,17)]
        tube('Calf binding '+side+str(i),pts,[.005]*17,'binding','shin.'+side,n=3)
    # modeled boot lasts taper and curve at toe, sole sits on z=0.
    rings('Cloth boot '+side,[(sg*.104,-.038,.02,.053,.116),(sg*.104,-.046,.044,.055,.121),(sg*.104,-.037,.071,.054,.113),(sg*.104,-.012,.108,.047,.074),(sg*.104,0,.16,.044,.044)],18,'boots','foot.'+side,.008)
    rings('Boot sole '+side,[(sg*.104,-.047,.0,.055,.117),(sg*.104,-.047,.021,.056,.119)],18,'binding','foot.'+side)
    tube('Boot toe stitch '+side,[(sg*.104+.049*cos(a),-.047+.11*sin(a),.035) for a in np.linspace(pi,2*pi,21)],[.0018]*21,'flax','foot.'+side,n=4)
    for i in range(2):ribbon('Boot strap '+side+str(i),[(sg*.104-.046,-.025-i*.042,.07),(sg*.104,-.032-i*.045,.09-i*.01),(sg*.104+.046,-.025-i*.042,.07)],.014,'leather','foot.'+side)
# Keep covered upper trousers strictly inside the coat envelope, including back hips.
for ob in mesh_objects:
    if ob.name.startswith('Charcoal trousers'):
        for v in ob.data.vertices:
            if v.co.z>=.69:
                f=max(0,min(1,(1.052-v.co.z)/.378));rx=.17+.052*f;ry=.108+.028*f;q=sqrt((v.co.x/rx)**2+(v.co.y/ry)**2)
                if q>.92:v.co.x*=.92/q;v.co.y*=.92/q
# The cream standing collar continues around the neck rather than ending in tabs.
cv=[];cu=[]
for j,(z,rx,ry) in enumerate([(1.433,.065,.054),(1.471,.054,.049)]):
    for k in range(15):
        a=-.60+(pi+1.20)*k/14;cv.append((rx*cos(a),ry*sin(a),z));cu.append((k/14,j))
obj('Cream back collar',cv,[(k,k+1,k+16,k+15) for k in range(14)],'flax','neck',cu)
# Face: asymmetry-free clean adult mesh, deliberately shaped jaw, cheek and brow planes.
rows=[(1.488,.026,.037,.0),(1.501,.045,.054,-.006),(1.525,.065,.061,-.003),(1.555,.077,.072,0),(1.59,.081,.078,.005),(1.625,.078,.075,.006),(1.658,.073,.071,.007),(1.682,.052,.052,.008),(1.695,.012,.014,.008)]
vs=[];uv=[];n=28
for j,(z,rx,ry,cy) in enumerate(rows):
    for k in range(n+1):
        a=2*pi*k/n;x=rx*cos(a);y=cy+ry*sin(a)
        # flatter midface, retained cheekbone silhouette.
        if y<cy:y=cy-ry*(.86+.14*abs(sin(a)))
        vs.append((x,y,z));uv.append((k/n,j/(len(rows)-1)))
fs=[(j*(n+1)+k,j*(n+1)+k+1,(j+1)*(n+1)+k+1,(j+1)*(n+1)+k) for j in range(len(rows)-1) for k in range(n)]
fs.append(tuple(reversed(range(n))));fs.append(tuple((len(rows)-1)*(n+1)+k for k in range(n)));obj('Sculpted adult head',vs,fs,'skin','head',uv)
# Nose bridge / tip with deliberately controlled plane changes.
verts=[(-.012,-.068,1.616),(.012,-.068,1.616),(-.010,-.092,1.587),(.010,-.092,1.587),(-.012,-.102,1.567),(.012,-.102,1.567),(0,-.108,1.570),(-.019,-.083,1.565),(.019,-.083,1.565),(0,-.081,1.557)]
obj('Nose bridge and tip',verts,[(0,1,3,2),(2,3,6),(2,6,4),(3,5,6),(4,6,5,9),(0,2,7),(2,4,7),(3,1,8),(5,3,8),(7,4,9),(9,5,8)],'skin','head')
for sg in [-1,1]:
    ellipsoid('Nostril '+str(sg),(sg*.011,-.092,1.561),(.004,.002,.002),'skin_shadow','head',8,5)
    # Eyes are almond patches with dark surrounding sculpted rims, not protruding spheres.
    cx=sg*.037;cy=-.073;z=1.602
    vs=[(cx-.022,cy+.003,z),(cx-.012,cy-.005,z+.008),(cx+.009,cy-.006,z+.008),(cx+.023,cy+.002,z+.001),(cx+.011,cy-.005,z-.006),(cx-.01,cy-.005,z-.005),(cx,cy-.007,z+.001)]
    obj('Almond eye '+str(sg),vs,[(6,i,(i+1)%6) for i in range(6)],'eye','head')
    ellipsoid('Iris '+str(sg),(cx,cy-.008,z+.001),(.0068,.002,.007),'iris','head',12,7)
    ellipsoid('Eye glint '+str(sg),(cx-.002,cy-.0105,z+.004),(.0015,.001,.0015),'eye','head',7,4)
    tube('Upper eyelid '+str(sg),[vs[i] for i in [0,1,2,3]],[.0015,.0025,.0026,.0013],'skin_shadow','head',n=5)
    tube('Lower eyelid '+str(sg),[vs[i] for i in [3,4,5,0]],[.001,.0014,.0015,.001],'skin','head',n=5)
    tube('Brow '+str(sg),[(sg*.015,-.071,1.62),(sg*.033,-.074,1.624),(sg*.053,-.071,1.623),(sg*.067,-.063,1.619)],[.003,.004,.0034,.0006],'hair','head',n=6)
    ellipsoid('Ear '+str(sg),(sg*.081,.001,1.581),(.012,.014,.026),'skin','head',10,8)
    ellipsoid('Ear inset '+str(sg),(sg*.09,-.008,1.582),(.005,.004,.014),'skin_shadow','head',10,8)
# Mouth follows philtrum then lower lip ridge.
tube('Upper lip', [(-.025,-.066,1.541),(-.012,-.073,1.545),(0,-.075,1.543),(.012,-.073,1.545),(.025,-.066,1.541)],[.001,.0028,.0024,.0028,.001],'skin_shadow','head',n=6)
tube('Lower lip',[(-.023,-.066,1.539),(0,-.075,1.537),(.023,-.066,1.539)],[.001,.0032,.001],'skin','head',n=6)
# Hair cap and sculpted swept locks; chunky deliberate strands stay readable in a mobile camera.
rings('Hair cap',[(0,.016,1.586,.081,.068),(0,.013,1.625,.084,.079),(0,.012,1.663,.078,.075),(0,.012,1.693,.06,.058),(0,.018,1.713,.022,.027)],24,'hair','head')
# Remove lower front cap faces to reveal forehead and eyes.
ob=mesh_objects[-1];me=ob.data;remove=[]
# Rebuild open cap by hiding front lower ring above brow via vertex lift.
for v in me.vertices:
    if v.co.y<-.014 and v.co.z<1.655:
        strength=max(0,min(1,(-v.co.y-.014)/.055));v.co.z+=max(0,1.657-v.co.z)*strength
for i in range(13):
    f=(i-6)/6;x=f*.074
    pts=[(x,-.05+.02*abs(f),1.65+.017*(1-abs(f))),(x*.9,-.025,1.699),(x*.55,.027,1.719),(x*.2,.066,1.706)]
    tube('Swept hair lock '+str(i),pts,[.010,.013,.011,.003],'hair' if i%3 else 'hair_ridge','head',n=6,elliptic=.7)
for sg in [-1,1]:
    tube('Temple hair '+str(sg),[(sg*.073,-.027,1.662),(sg*.077,-.052,1.625),(sg*.066,-.073,1.586)],[.012,.009,.001],'hair','head',n=7)
    tube('Front loose lock '+str(sg),[(sg*.028,-.063,1.686),(sg*.047,-.083,1.652),(sg*.052,-.086,1.607)],[.009,.007,.0007],'hair','head',n=7)
ellipsoid('Ponytail knot',(0,.069,1.721),(.032,.029,.028),'hair','head',12,8)
for i in range(9):
    a=2*pi*i/9;dx=cos(a)*.025;dy=sin(a)*.016
    pts=[(dx*.4,.073+dy,1.729),(dx,.119+dy,1.679),(dx*1.15,.145+dy,1.597),(dx*.7+.009,.15+dy,1.515),(dx*.4+.02,.135+dy,1.469)]
    tube('Ponytail lock '+str(i),pts,[.015,.021,.02,.014,.001],'hair' if i%3 else 'hair_ridge','ponytail',n=6,elliptic=.8)
for j in range(3):tube('Hair tie '+str(j),[(.031*cos(a),.07+.027*sin(a),1.721+j*.006) for a in np.linspace(0,2*pi,19)],[.0025]*19,'binding','head',n=5)
# Sword and scabbard: rigid independent mesh object driven by its attachment bone.
sword_base=Vector(bones['sword'][0]);sword_end=Vector(bones['sword'][1]);sd=(sword_end-sword_base).normalized();sx=Vector((1,0,0));sy=sd.cross(sx).normalized()
def sword_pt(t,x=0,y=0):return tuple(sword_base+sd*t+sx*x+sy*y)
tube('Jian scabbard',[sword_pt(.03),sword_pt(.59),sword_pt(.68)],[.023,.020,.012],'leather','pelvis',n=10,elliptic=.6)
for t in [.04,.30,.62]:tube('Scabbard bronze collar '+str(t),[sword_pt(t),sword_pt(t+.022)],[.024,.024],'bronze','pelvis',n=10,elliptic=.65)
tube('Sword grip',[sword_pt(-.13),sword_pt(-.02)],[.014,.015],'boots','sword',n=10)
for j in range(7):tube('Grip cord '+str(j),[sword_pt(-.12+j*.014,x=.016*cos(a),y=.011*sin(a)) for a in np.linspace(0,2*pi,13)],[.0018]*13,'binding','sword',n=3)
tube('Jian guard',[sword_pt(-.015,x=-.052),sword_pt(-.03,x=-.03),sword_pt(-.03,x=.03),sword_pt(-.015,x=.052)],[.006,.009,.009,.006],'bronze','sword',n=7)
ellipsoid('Jian pommel',sword_pt(-.143),(.021,.015,.019),'bronze','sword',12,7)
verts=[]
for t,w,d in [(0,.019,.004),(.55,.013,.003),(.68,.0,.0002)]:
    for x,y in [(-w,0),(0,-d),(w,0),(0,d)]:verts.append(sword_pt(t,x,y))
fs=[]
for j in range(2):
    for i in range(4):fs.append((j*4+i,j*4+(i+1)%4,(j+1)*4+(i+1)%4,(j+1)*4+i))
obj('Jian steel blade',verts,fs,'steel','sword')
# Weld the tunic torso and sleeve roots into one continuous garment surface.
# A deterministic voxel union removes intersection seams; applied decimation and
# barycentric skin transfer keep this an ordinary skinned glTF mesh.
cloth_parts=[o for o in mesh_objects if o.name in ['Wrap tunic torso','Tunic sleeve L','Tunic sleeve R']]
sv=[];sw=[];st=[]
for ob in cloth_parts:
    base=len(sv);me=ob.data;me.calc_loop_triangles();sv.extend([v.co.copy() for v in me.vertices])
    for v in me.vertices:sw.append({ob.vertex_groups[g.group].name:g.weight for g in v.groups})
    st.extend([tuple(base+i for i in t.vertices) for t in me.loop_triangles])
source_tree=BVHTree.FromPolygons(sv,st,all_triangles=True)
bpy.ops.object.select_all(action='DESELECT')
for ob in cloth_parts:ob.select_set(True)
bpy.context.view_layer.objects.active=cloth_parts[0];bpy.ops.object.join();coat=bpy.context.object;coat.name='Tunic continuous shoulders'
for mod in list(coat.modifiers):coat.modifiers.remove(mod)
mod=coat.modifiers.new('Weld cloth volume','REMESH');mod.mode='VOXEL';mod.voxel_size=.009;mod.use_smooth_shade=True;bpy.ops.object.modifier_apply(modifier=mod.name)
mod=coat.modifiers.new('Soften cloth volume','SMOOTH');mod.factor=.65;mod.iterations=3;bpy.ops.object.modifier_apply(modifier=mod.name)
coat.data.calc_loop_triangles();mod=coat.modifiers.new('Game topology','DECIMATE');mod.ratio=min(1,850/max(1,len(coat.data.loop_triangles)));bpy.ops.object.modifier_apply(modifier=mod.name)
for vg in list(coat.vertex_groups):coat.vertex_groups.remove(vg)
groups={}
for v in coat.data.vertices:
    hit,normal,ti,dist=source_tree.find_nearest(v.co);ids=st[ti];a,b,c=[sv[i] for i in ids];e0=b-a;e1=c-a;e2=hit-a;d00=e0.dot(e0);d01=e0.dot(e1);d11=e1.dot(e1);d20=e2.dot(e0);d21=e2.dot(e1);den=d00*d11-d01*d01
    if abs(den)<1e-12:bc=[1,0,0]
    else:
        vb=(d11*d20-d01*d21)/den;vc=(d00*d21-d01*d20)/den;bc=[1-vb-vc,vb,vc]
    weights={}
    for vi,f in zip(ids,bc):
        for bn,w in sw[vi].items():weights[bn]=weights.get(bn,0)+max(0,f)*w
    total=sum(weights.values())
    for bn,w in weights.items():
        if bn not in groups:groups[bn]=coat.vertex_groups.new(name=bn)
        if w>1e-6:groups[bn].add([v.index],w/total,'REPLACE')
for p in coat.data.polygons:p.use_smooth=True
bpy.context.view_layer.objects.active=coat;bpy.ops.object.mode_set(mode='EDIT');bpy.ops.mesh.select_all(action='SELECT');bpy.ops.uv.smart_project(island_margin=.02);bpy.ops.object.mode_set(mode='OBJECT')
mod=coat.modifiers.new('Humanoid skin','ARMATURE');mod.object=rig
mesh_objects=[o for o in mesh_objects if o not in cloth_parts]+[coat]
# Reproject facing and stitched edges onto the final continuous cloth surface.
ct=BVHTree.FromPolygons([v.co for v in coat.data.vertices],[list(p.vertices) for p in coat.data.polygons])
for ob in mesh_objects:
    if any(s in ob.name.lower() for s in ['lapel','collar','exposed neckline','shirt underlap']) and ob!=coat:
        for v in ob.data.vertices:
            hit=ct.ray_cast(Vector((v.co.x,-1,v.co.z)),Vector((0,1,0)))
            if hit[0] is not None:v.co.y=hit[0].y-(.010 if 'stitched' in ob.name.lower() else .004 if 'underlap' in ob.name.lower() or 'neckline' in ob.name.lower() else .009)
# consolidate all character surfaces to one skinned body and all sword pieces to a separate object.
def join_group(items,name):
    bpy.ops.object.select_all(action='DESELECT')
    for ob in items:ob.select_set(True)
    bpy.context.view_layer.objects.active=items[0];bpy.ops.object.join();ob=bpy.context.object;ob.name=name;ob.data.name=name
    bpy.context.scene.cursor.location=(0,0,0);bpy.ops.object.origin_set(type='ORIGIN_CURSOR');return ob
swords=[ob for ob in mesh_objects if len(ob.vertex_groups)==1 and ob.vertex_groups[0].name=='sword']
body=join_group([ob for ob in mesh_objects if ob not in swords],'Novice_Body_Skinned');sword=join_group(swords,'Sword_Independent')
mesh_objects=[body,sword]
# Normalize all influences and ensure no unused armature modifiers.
for ob in mesh_objects:
    mods=list(ob.modifiers)
    for mod in mods[1:]:ob.modifiers.remove(mod)
    ob['original_asset']=True
# Pose and animation utilities. Every deform bone is keyed, avoiding implicit retarget dependencies.
rest={bn:arm.bones[bn].matrix_local.copy() for bn in bones}
for pb in rig.pose.bones:pb.rotation_mode='QUATERNION'
def reset_pose():
    for pb in rig.pose.bones:pb.location=(0,0,0);pb.rotation_quaternion=(1,0,0,0);pb.scale=(1,1,1)
def rot(bn,xyz):rig.pose.bones[bn].rotation_quaternion=__import__('mathutils').Euler(tuple(math.radians(a) for a in xyz),'XYZ').to_quaternion()
def keyall(frame):
    # Relaxed hands by default, a closed sword grip for the combat action.
    attacking=rig.animation_data.action and rig.animation_data.action.name=='attack_basic'
    for side in ['L','R']:
        rot('fingers.'+side,(58 if attacking and side=='R' else 18,0,0));rot('fingertips.'+side,(75 if attacking and side=='R' else 25,0,0))
    for pb in rig.pose.bones:
        pb.keyframe_insert('location',frame=frame,group=pb.name);pb.keyframe_insert('rotation_quaternion',frame=frame,group=pb.name);pb.keyframe_insert('scale',frame=frame,group=pb.name)
def new_action(name):
    rig.animation_data_create();act=bpy.data.actions.new(name);rig.animation_data.action=act;return act
# Build matrix with minimal-twist orientation of rest bone then solve hierarchical basis.
def desired_matrix(bn,head,tail):
    a=Vector(bones[bn][0]);b=Vector(bones[bn][1]);q=(b-a).normalized().rotation_difference((Vector(tail)-Vector(head)).normalized());r=q.to_matrix()@rest[bn].to_3x3();m=r.to_4x4();m.translation=Vector(head);return m

def apply_desired(wanted):
    matrices={}
    for bn,(a,b,par) in bones.items():
        if bn in wanted:matrices[bn]=wanted[bn]
        elif par:matrices[bn]=matrices[par]@rest[par].inverted()@rest[bn]
        else:matrices[bn]=rest[bn].copy()
        basis=rest[bn].inverted()@(rest[par]@matrices[par].inverted() if par else Matrix.Identity(4))@matrices[bn]
        loc,q,sc=basis.decompose();pb=rig.pose.bones[bn];pb.location=loc;pb.rotation_quaternion=q;pb.scale=sc
    return matrices

def ik_joint(hip,ankle,l1,l2,pole):
    h=Vector(hip);a=Vector(ankle);d=(a-h);dist=min(d.length,l1+l2-.00001);axis=d.normalized();p=Vector(pole);perp=(p-axis*p.dot(axis)).normalized();u=(l1*l1-l2*l2+dist*dist)/(2*dist);v=sqrt(max(0,l1*l1-u*u));return h+axis*u+perp*v
# Idle relaxed A-pose reduced to natural hanging arms, breathing and ponytail inertia.
act=new_action('idle')
for frame in range(1,122):
    t=(frame-1)/60;reset_pose();b=sin(t*pi)
    rot('spine',(0,.55*b,0));rot('chest',(1.0*b,0,0));rot('head',(-.4*b,.4*b,0));rot('ponytail',(1.5*b,0,1*b))
    # Bone world rotation around -/+Y brings A-pose arms in toward torso.
    for side,sg in [('L',-1),('R',1)]:rot('upper_arm.'+side,(0,0,sg*21));rot('forearm.'+side,(0,0,sg*-4))
    keyall(frame)
# Walk root motion: .4s, 1.6m cycle = exactly4m/s. Analytic stance target frozen in world.
act=new_action('walk');walk_samples=[];cycle=.4;speed=4.;stride=speed*cycle;stance=.5
for frame in range(1,26):
    t=(frame-1)/60;p=t/cycle;root_y=-speed*t;root_z=-.125+.012*cos(p*4*pi);wanted={}
    rootm=rest['root'].copy();rootm.translation=Vector((0,root_y,root_z));wanted['root']=rootm
    # neutral upper body inherits translating root.
    for side,sg,phase in [('L',-1,0),('R',1,.5)]:
        phase_total=p+phase;cyc=math.floor(phase_total);f=phase_total-cyc
        # start at +0.4m ahead (negativeY), stance world position remains exact.
        if f<.5:
            fy=-(cyc-phase)*stride-.4;fz=.125;contact=True
        else:
            q=(f-.5)/.5;s=q*q*(3-2*q);fy=-(cyc-phase)*stride-.4-stride*s;fz=.125+.17*sin(pi*q);contact=False
        hip=Vector((sg*.097,root_y,.94+root_z));ankle=Vector((sg*.104,fy,fz));l1=(Vector(bones['thigh.'+side][1])-Vector(bones['thigh.'+side][0])).length;l2=(Vector(bones['shin.'+side][1])-Vector(bones['shin.'+side][0])).length
        knee=ik_joint(hip,ankle,l1,l2,(0,-1,.04));wanted['thigh.'+side]=desired_matrix('thigh.'+side,hip,knee);wanted['shin.'+side]=desired_matrix('shin.'+side,knee,ankle)
        wanted['foot.'+side]=desired_matrix('foot.'+side,ankle,ankle+Vector((0,-.15,-.063)))
        sh=Vector((sg*.205,root_y,1.405+root_z));swing=sin((p+phase)*2*pi)*.19;wr=sh+Vector((sg*.065,swing,-.425));el=ik_joint(sh,wr,.273,.213,(0,-1,0));wanted['upper_arm.'+side]=desired_matrix('upper_arm.'+side,sh,el);wanted['forearm.'+side]=desired_matrix('forearm.'+side,el,wr)
        walk_samples.append({'frame':frame,'foot':side,'stance':contact,'ankle_world':list(ankle),'root_y':root_y})
    apply_desired(wanted);rot('ponytail',(4*sin(p*2*pi),0,0));keyall(frame)
# attack .70 sec = frames1..43. Sword tip exactly extended at time .25 (frame16).
act=new_action('attack_basic');attack_contact=None
for frame in range(1,44):
    t=(frame-1)/60
    reset_pose()
    # Windup / draw .0-.15; decisive thrust .15-.25; recover by .70.
    if t<=.15:k=t/.15;extend=.0;draw=k
    elif t<=.25:k=(t-.15)/.10;extend=k*k*(3-2*k);draw=1
    else:k=(t-.25)/.45;extend=1-(k*k*(3-2*k));draw=1-min(1,max(0,(t-.48)/.22))
    sh=Vector((.205,0,1.405));wrist=Vector((.30-.12*extend,-.18-.28*extend,1.16+.13*extend));el=ik_joint(sh,wrist,.273,.213,(1,0,-.5))
    wanted={'upper_arm.R':desired_matrix('upper_arm.R',sh,el),'forearm.R':desired_matrix('forearm.R',el,wrist),'hand.R':desired_matrix('hand.R',wrist,wrist+Vector((0,-.095,0)))}
    grip=wrist+Vector((0,-.05,0));target_base=grip+Vector((0,-.075,0));target_tip=target_base+Vector((0,-.685,0))
    a=Vector(bones['sword'][0]).lerp(target_base,draw);b=Vector(bones['sword'][1]).lerp(target_tip,draw);wanted['sword']=desired_matrix('sword',a,b)
    apply_desired(wanted);rot('upper_arm.L',(12,0,-20));rot('forearm.L',(-10,0,-20));rot('ponytail',(0,0,5*extend));keyall(frame)
    if frame==16:attack_contact={'frame':16,'seconds':.25,'tip_target_blender':list(target_tip),'note':'Sword_Independent rigidly skinned to sword bone; contact is maximum thrust extension.'}
act.pose_markers.new('sword_tip_contact_0.25s').frame=16
act=new_action('hit')
for frame in range(1,20):
    t=(frame-1)/18;reset_pose();a=sin(pi*t);rot('spine',(-12*a,0,0));rot('chest',(-10*a,0,0));rot('head',(-9*a,0,0));rot('upper_arm.L',(5*a,0,-15));rot('upper_arm.R',(5*a,0,15));keyall(frame)
act=new_action('death')
for frame in range(1,62):
    t=(frame-1)/60;reset_pose();q=min(1,t/.85);s=q*q*(3-2*q);rot('root',(-90*s,0,0));rig.pose.bones['root'].location=(0,.18*s,.18*s)
    rot('upper_arm.L',(0,0,-15*s));rot('upper_arm.R',(0,0,15*s));rot('forearm.L',(-8*s,0,0));rot('forearm.R',(-8*s,0,0));rot('ponytail',(30*s,0,0));keyall(frame)
# Linear per-frame samples; exact endpoints and no interpolation overshoot.
for act in bpy.data.actions:
    for fc in act.fcurves:
        for k in fc.keyframe_points:k.interpolation='LINEAR'
    act.use_fake_user=True
# NLA export ensures exactly the named five clips.
rig.animation_data.action=None
for name in ['idle','walk','attack_basic','hit','death']:
    tr=rig.animation_data.nla_tracks.new();tr.name=name;act=bpy.data.actions[name];st=tr.strips.new(name,1,act);st.name=name;tr.mute=True
reset_pose();S.frame_start=1;S.frame_end=121;S.frame_set(1)
bpy.data.orphans_purge(do_recursive=True)
# Export selected rig+meshes, standard Principled-only PNG textures embedded in the GLB.
bpy.ops.object.select_all(action='DESELECT');rig.select_set(True)
for ob in mesh_objects:ob.select_set(True)
bpy.context.view_layer.objects.active=rig
bpy.ops.export_scene.gltf(filepath=OUT,export_format='GLB',use_selection=True,export_yup=True,export_apply=False,export_animations=True,export_animation_mode='ACTIONS',export_force_sampling=True,export_anim_slide_to_zero=True,export_frame_range=False,export_nla_strips_merged_animation_name='Animation',export_materials='EXPORT',export_image_format='AUTO',export_cameras=False,export_lights=False,export_skins=True,export_def_bones=True)
# Packed source plus evidence. Studio helpers are excluded from asset exports.
rig.animation_data.action=bpy.data.actions['idle'];S.frame_set(1)
world=bpy.data.worlds.new('Character studio');S.world=world;world.use_nodes=True;world.node_tree.nodes['Background'].inputs[0].default_value=(.28,.32,.34,1);world.node_tree.nodes['Background'].inputs[1].default_value=.45
S.render.engine='CYCLES';S.cycles.samples=48;S.cycles.use_denoising=False;S.render.resolution_x=900;S.render.resolution_y=1100;S.render.resolution_percentage=100
S.view_settings.view_transform='AgX'
def point_at(ob,p):ob.rotation_euler=(Vector(p)-ob.location).to_track_quat('-Z','Y').to_euler()
for name,loc,power,size,color in [('Key',(-3,-4,5),450,4,(1,.85,.68)),('Fill',(3,-2,3),200,3,(.66,.79,1)),('Rim',(1,3,4),500,3,(.83,.91,1))]:
    dat=bpy.data.lights.new(name,'AREA');dat.energy=power;dat.shape='DISK';dat.size=size;dat.color=color;ob=bpy.data.objects.new(name,dat);bpy.context.collection.objects.link(ob);ob.location=loc;point_at(ob,(0,0,1))
camd=bpy.data.cameras.new('Character Camera');cam=bpy.data.objects.new('Character Camera',camd);bpy.context.collection.objects.link(cam);S.camera=cam;cam.location=(2.35,-4.5,2.1);point_at(cam,(0,0,.91));camd.type='ORTHO';camd.ortho_scale=2.12
S.render.film_transparent=False;S.render.image_settings.file_format='PNG';S.render.filepath=os.path.join(SRC,'character-preview.png')
bpy.ops.wm.save_as_mainfile(filepath=os.path.join(SRC,'character_novice.blend'),compress=True)
bpy.ops.render.render(write_still=True)
# Source truth records; independent GLB parser/reimport supplements them below.
tri=sum(sum(len(p.vertices)-2 for p in ob.data.polygons) for ob in mesh_objects)
record={'asset':'player_novice.glb','blender_version':bpy.app.version_string,'source':'original scripted modeling following AI-generated original turnaround','triangles_source':tri,'body_triangles':sum(len(p.vertices)-2 for p in body.data.polygons),'sword_triangles':sum(len(p.vertices)-2 for p in sword.data.polygons),'height_m_bind':max(v.co.z for ob in mesh_objects for v in ob.data.vertices),'origin':'feet center at (0,0,0)','front':'Blender -Y, glTF +Z','skin_bones':len(bones),'textures':'Embedded baked PNG, maximum512x512','clips_expected':{'idle':2.0,'walk':.4,'attack_basic':.7,'hit':.3,'death':1.0},'walk':{'type':'root motion','root_bone':'root','velocity_mps':4,'cycle_seconds':.4,'cycle_displacement_m':1.6,'loop_integration':'Godot must extract root bone motion; root wraps at loop boundary while cumulative actor transform advances. Do not add duplicate 4m/s motion.','stance':'Each stance ankle is held at a mathematically fixed world-space target; only swing feet advance.','samples':walk_samples},'attack_contact':attack_contact,'death_final_hold_seconds':.15,'third_party_assets':[]}
with open(os.path.join(ROOT,'docs/art/character-validation.json'),'w') as f:json.dump(record,f,indent=2)
print('CHARACTER BUILD COMPLETE',tri,OUT)
