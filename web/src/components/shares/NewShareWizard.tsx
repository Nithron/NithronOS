import { useEffect, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import http from '@/lib/nos-client';
import { ShareForm, ShareFormInput } from '@/features/shares/schemas';

export default function NewShareWizard({ open, onClose }:{open:boolean; onClose:(created:boolean)=>void}) {
  const [roots,setRoots]=useState<string[]>([]);
  const [available,setAvailable]=useState<string[]>([]);
  const [newU,setNewU]=useState('');
  const [newP,setNewP]=useState('');
  const [step,setStep]=useState(1);
  const schema = useMemo(()=>ShareForm(roots),[roots]);
  const { register, reset, formState:{errors}, watch, setValue, getValues } = useForm<ShareFormInput>({ resolver:zodResolver(schema), defaultValues:{ type:'smb', ro:false, users:[] } });

  useEffect(()=>{ if(open){ (async()=>{ const roots = await http.pools.roots() as string[]; setRoots(roots); })(); } },[open]);
  const type = watch('type');
  useEffect(()=>{ (async()=>{ if(open && type==='smb'){ const list = await http.smb.usersList() as string[]; setAvailable(list); } })(); },[open, type]);

  const next = ()=> {
    setStep(s => {
      if (s === 1) return 2
      if (s === 2) return type === 'smb' ? 3 : 4
      if (s === 3) return 4
      return 4
    })
  };
  const back = ()=> {
    setStep(s => {
      if (s === 4) return type === 'smb' ? 3 : 2
      if (s === 3) return 2
      if (s === 2) return 1
      return 1
    })
  };
  const createShare = async () => {
    const vals = getValues()
    if (vals.type === 'smb' && (!vals.users || vals.users.length === 0)) {
      if (!confirm('No SMB users selected — continue?')) return
    }
    try {
      await http.shares.create(vals)
      alert('Share created')
      close(true)
    } catch (err:any) {
      alert(err?.message || 'Failed to create share')
    }
  }
  const close = (ok=false)=>{ reset(); setStep(1); onClose(ok); };

  if(!open) return null;
  return (
    <div className="modal">
      <div className="card w-[680px] p-6 space-y-4">
        <h2 className="text-xl font-semibold">New Share</h2>
        {step===1 && (
          <div className="space-y-3">
            <label className="block">Type
              <select {...register('type')} className="input"><option value="smb">SMB</option><option value="nfs">NFS</option></select>
            </label>
            <label className="block">Name
              <input {...register('name')} className="input" placeholder="media"/>
              {errors.name && <p className="text-red-500 text-sm">{errors.name.message as string}</p>}
            </label>
            <label className="inline-flex items-center gap-2"><input type="checkbox" {...register('ro')}/> Read-only</label>
          </div>
        )}
        {step===2 && (
          <div className="space-y-3">
            <label className="block">Mount point
              <select {...register('path')} className="input">
                <option value="">Select...</option>
                {roots.map(r=> <option key={r} value={r}>{r}</option>)}
              </select>
            </label>
            {errors.path && <p className="text-red-500 text-sm">{errors.path.message as string}</p>}
            <small className="text-muted-foreground">You can append a subfolder in the next step.</small>
          </div>
        )}
        {step===3 && type==='smb' && (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">Select SMB users allowed to access this share.</p>
            <div className="max-h-40 overflow-auto border rounded p-2 space-y-1">
              {available.map(u=>(
                <label key={u} className="flex items-center gap-2">
                  <input type="checkbox" checked={watch('users')?.includes(u)} onChange={(e)=>{
                    const cur = new Set(watch('users')||[]);
                    if (e.target.checked) cur.add(u); else cur.delete(u);
                    setValue('users', Array.from(cur));
                  }}/>
                  <span className="font-mono">{u}</span>
                </label>
              ))}
            </div>
            <div className="flex gap-2">
              <input className="input" placeholder="new username" value={newU} onChange={e=>setNewU(e.target.value)}/>
              <input className="input" placeholder="password (optional)" value={newP} onChange={e=>setNewP(e.target.value)} type="password"/>
              <button className="btn" onClick={async()=>{
                if(!newU) return;
                await http.smb.userCreate({username:newU, password:newP||undefined});
                // refresh from server to reflect any normalization
                const list = await http.smb.usersList();
                setAvailable(list as string[]);
                setValue('users', Array.from(new Set([...(watch('users')||[]), newU])));
                setNewU(''); setNewP('');
              }}>Add</button>
            </div>
          </div>
        )}
        {step===3 && type==='nfs' && (
          <p className="text-sm text-muted-foreground">Users not required for NFS</p>
        )}
        {step===4 && (
          <div className="space-y-2 text-sm">
            <div className="grid grid-cols-5 gap-2">
              <div className="text-muted-foreground">Type</div>
              <div className="col-span-4 uppercase">{watch('type')}</div>
              <div className="text-muted-foreground">Name</div>
              <div className="col-span-4">{watch('name')}</div>
              <div className="text-muted-foreground">Path</div>
              <div className="col-span-4 font-mono">{watch('path')}</div>
              <div className="text-muted-foreground">Users</div>
              <div className="col-span-4">{(watch('users')||[]).length}</div>
              <div className="text-muted-foreground">Read-only</div>
              <div className="col-span-4">{watch('ro') ? 'Yes' : 'No'}</div>
            </div>
          </div>
        )}
        {step===2 && (
          <div className="space-y-3">
            <label className="block">Mount point
              <select {...register('path')} className="input">
                <option value="">Select...</option>
                {roots.map(r=> <option key={r} value={r}>{r}</option>)}
              </select>
            </label>
            <small className="text-muted-foreground">You can append a subfolder in the next step.</small>
          </div>
        )}
        <div className="flex justify-between">
          <button className="btn-ghost" onClick={()=>close(false)}>Cancel</button>
          <div className="space-x-2">
            {step>1 && <button className="btn-outline" onClick={back}>Back</button>}
            {step<4 ? (
              <button className="btn" onClick={next}>Next</button>
            ) : (
              <button className="btn" onClick={createShare}>Create</button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}


