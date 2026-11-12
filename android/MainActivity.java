package com.anr2.mm2;

import android.app.Activity;
import android.app.Dialog;
import android.graphics.Color;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.InputType;
import android.text.Spannable;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.zip.ZipEntry;
import java.util.zip.ZipInputStream;
import org.json.JSONObject;
import com.anr2.mm2.mobile.EbitenView;
import com.anr2.mm2.mobile.Mobile;
import go.Seq;

public final class MainActivity extends Activity {
    private final java.util.concurrent.ExecutorService engineTasks=java.util.concurrent.Executors.newSingleThreadExecutor();
    private android.graphics.Bitmap borderBitmap;
    private FrameLayout root;
    private EbitenView gameView;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private boolean resumed, ready, destroyed;
    private Dialog editorDialog;
    private long seenInput = -1;
    private int polls;
    private final Runnable poll = new Runnable() {
        @Override public void run() {
            if (destroyed) return;
            if (ready && resumed) {
                try {
                    String request = Mobile.inputRequest();
                    if (!request.isEmpty()) {
                        JSONObject data = new JSONObject(request);
                        long token = data.getLong("token");
                        if (token != seenInput && editorDialog == null) { seenInput=token; showEditor(data); }
                    } else if (editorDialog != null) { editorDialog.dismiss(); editorDialog=null; }
                    if (++polls % 8 == 0) android.util.Log.i("MM2_STATE", Mobile.diagnostics());
                } catch (Exception e) { android.util.Log.e("MM2", "Input bridge", e); }
            }
            handler.postDelayed(this, 200);
        }
    };

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        getWindow().setFlags(WindowManager.LayoutParams.FLAG_FULLSCREEN, WindowManager.LayoutParams.FLAG_FULLSCREEN);
        getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY|View.SYSTEM_UI_FLAG_HIDE_NAVIGATION|View.SYSTEM_UI_FLAG_FULLSCREEN|View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
        root=new FrameLayout(this); root.setBackgroundColor(Color.rgb(19,25,35)); setContentView(root);
        root.setOnApplyWindowInsetsListener((v,insets)-> {
            int left=insets.getSystemWindowInsetLeft(), top=insets.getSystemWindowInsetTop(), right=insets.getSystemWindowInsetRight(), bottom=insets.getSystemWindowInsetBottom();
            if (android.os.Build.VERSION.SDK_INT >= 28 && insets.getDisplayCutout()!=null) {
                android.view.DisplayCutout cut=insets.getDisplayCutout();
                left=Math.max(left,cut.getSafeInsetLeft());right=Math.max(right,cut.getSafeInsetRight());top=Math.max(top,cut.getSafeInsetTop());bottom=Math.max(bottom,cut.getSafeInsetBottom());
            }
            v.setPadding(left,top,right,bottom);return insets;
        });
        TextView loading=new TextView(this);loading.setText("MM2");loading.setTextColor(0xffe9cd96);loading.setTextSize(28);loading.setGravity(Gravity.CENTER);
        root.addView(loading,new FrameLayout.LayoutParams(-1,-1));
        Seq.setContext(getApplicationContext());
        new Thread(()-> {
            try {
                File runtime=prepareRuntime();
                android.util.Log.i("MM2","Runtime extraction complete; loading game session");
                Mobile.setDisplayDensity(getResources().getDisplayMetrics().density);
                Mobile.initialize(runtime.getAbsolutePath(),new File(getFilesDir(),"player").getAbsolutePath());
                android.util.Log.i("MM2","Game session ready; creating renderer");
                runOnUiThread(()->startView(loading));
            } catch (Exception e) {
                android.util.Log.e("MM2", "Initialization", e);
                runOnUiThread(()->loading.setText("MM2\n"+e.getMessage()));
            }
        },"mm2-assets").start();
        handler.post(poll);
    }

    private void startView(View loading) {
        if(destroyed)return;
        gameView=new EbitenView(this) {
            @Override protected void onErrorOnGameUpdate(Exception e) { android.util.Log.e("MM2", "Game update", e); }
            @Override public boolean dispatchTouchEvent(MotionEvent e) {
                android.util.Log.i("MM2_TOUCH","game "+e.getActionMasked());
                int action=e.getActionMasked(),index=e.getActionIndex();
                float scale=Math.min(getWidth()/844f,getHeight()/390f);
                if(scale<=0)return true;
                float ox=(getWidth()-844*scale)/2,oy=(getHeight()-390*scale)/2;
                if(action==MotionEvent.ACTION_CANCEL){Mobile.cancelPointer();return true;}
                long eventTime=e.getEventTime();
                if(action==MotionEvent.ACTION_MOVE){for(int i=0;i<e.getPointerCount();i++)Mobile.pointer(e.getPointerId(i),(e.getX(i)-ox)/scale,(e.getY(i)-oy)/scale,1,eventTime);return true;}
                if(action==MotionEvent.ACTION_DOWN||action==MotionEvent.ACTION_POINTER_DOWN||action==MotionEvent.ACTION_UP||action==MotionEvent.ACTION_POINTER_UP){
                    int phase=(action==MotionEvent.ACTION_DOWN||action==MotionEvent.ACTION_POINTER_DOWN)?0:2;
                    Mobile.pointer(e.getPointerId(index),(e.getX(index)-ox)/scale,(e.getY(index)-oy)/scale,phase,eventTime);return true;
                }
                return super.dispatchTouchEvent(e);
            }
        };
        root.removeView(loading);root.addView(gameView,new FrameLayout.LayoutParams(-1,-1));ready=true;
        gameView.addOnLayoutChangeListener((v,l,t,r,b,ol,ot,or,ob)->{
            float density=getResources().getDisplayMetrics().density;
            Mobile.setViewportDp((r-l)/density,(b-t)/density);
        });
        if(resumed)engineTasks.execute(()->Mobile.resume());else engineTasks.execute(()->{Mobile.pause();handler.post(()->{if(!destroyed&&!resumed)gameView.suspendGame();});});
    }

    private void showEditor(JSONObject data) throws Exception {
        final long token=data.getLong("token");
        final Dialog dialog=new Dialog(this);dialog.requestWindowFeature(Window.FEATURE_NO_TITLE);
        if(borderBitmap==null)borderBitmap=android.graphics.BitmapFactory.decodeFile(new File(getFilesDir(),"runtime/workplace/hd-amiga-r1/frame.png").getPath());
        LinearLayout panel=new LinearLayout(this);panel.setOrientation(LinearLayout.VERTICAL);panel.setPadding(dp(22),dp(15),dp(22),dp(14));panel.setBackground(new OrnateBackground());
        TextView title=new TextView(this);title.setText(data.getString("title"));title.setTextColor(0xffe9cd96);title.setTextSize(19);panel.addView(title);
        EditText edit=new EditText(this);edit.setId(0x4d3201);edit.setSingleLine(true);edit.setTextColor(0xffe9d7b7);edit.setTextSize(18);edit.setInputType(InputType.TYPE_CLASS_TEXT);
        edit.setImeOptions(EditorInfo.IME_ACTION_DONE|EditorInfo.IME_FLAG_NO_EXTRACT_UI|EditorInfo.IME_FLAG_NO_FULLSCREEN);
        edit.setBackgroundTintList(android.content.res.ColorStateList.valueOf(0xffb99a5d));
        edit.setText(data.getString("value"));edit.setSelection(edit.length());
        LinearLayout.LayoutParams editorParams=new LinearLayout.LayoutParams(-1,dp(52));editorParams.topMargin=dp(10);panel.addView(edit,editorParams);
        LinearLayout row=new LinearLayout(this);row.setOrientation(LinearLayout.HORIZONTAL);
        Button submit=new Button(this),cancel=new Button(this);submit.setText(data.getString("confirm"));cancel.setText(data.getString("cancel"));
        for(Button button:new Button[]{submit,cancel}){button.setTextColor(0xffe9cd96);button.setBackground(new OrnateBackground());LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,dp(52),1);lp.setMargins(dp(3),dp(8),dp(3),0);row.addView(button,lp);}
        panel.addView(row);dialog.setContentView(panel);dialog.setCanceledOnTouchOutside(false);
        Runnable accept=()->{edit.clearComposingText();String text=edit.getText().toString();submit.setEnabled(false);cancel.setEnabled(false);engineTasks.execute(()->{boolean ok=Mobile.submitInput(text,token,true);String error=Mobile.inputError();handler.post(()->{if(destroyed)return;if(ok){dialog.dismiss();editorDialog=null;}else{submit.setEnabled(true);cancel.setEnabled(true);edit.setError(error);}});});};
        submit.setOnClickListener(v->accept.run());
        cancel.setOnClickListener(v->{engineTasks.execute(()->Mobile.submitInput("",token,false));dialog.dismiss();editorDialog=null;});
        edit.setOnEditorActionListener((v,action,event)-> {
            if(action==EditorInfo.IME_ACTION_DONE || (event!=null&&event.getKeyCode()==android.view.KeyEvent.KEYCODE_ENTER)) {
                Spannable text=edit.getText();
                if(BaseInputConnection.getComposingSpanStart(text)>=0)return true;
                accept.run();return true;
            }
            return false;
        });
        dialog.setOnCancelListener(v->{engineTasks.execute(()->Mobile.submitInput("",token,false));editorDialog=null;});
        editorDialog=dialog;dialog.show();
        Window window=dialog.getWindow();
        window.setBackgroundDrawable(new android.graphics.drawable.ColorDrawable(Color.TRANSPARENT));
        int available=getResources().getDisplayMetrics().widthPixels;
        window.setLayout(Math.min(dp(468),available-dp(40)),WindowManager.LayoutParams.WRAP_CONTENT);
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE|WindowManager.LayoutParams.SOFT_INPUT_STATE_ALWAYS_VISIBLE);
        edit.requestFocus();edit.postDelayed(()->((InputMethodManager)getSystemService(INPUT_METHOD_SERVICE)).showSoftInput(edit,InputMethodManager.SHOW_IMPLICIT),150);
    }

    private int dp(float value){return Math.round(value*getResources().getDisplayMetrics().density);}
    @Override public boolean dispatchTouchEvent(MotionEvent event){android.util.Log.i("MM2_TOUCH","window "+event.getActionMasked());return super.dispatchTouchEvent(event);}
    private final class OrnateBackground extends android.graphics.drawable.Drawable {
        private final android.graphics.Paint paint=new android.graphics.Paint(android.graphics.Paint.FILTER_BITMAP_FLAG);
        @Override public void draw(android.graphics.Canvas canvas){
            android.graphics.Rect r=getBounds();paint.setColor(0xff121b24);canvas.drawRect(r,paint);if(borderBitmap==null)return;
            int b=dp(8),w=borderBitmap.getWidth(),h=borderBitmap.getHeight();int[] sx={0,34,w-34,w},sy={0,34,h-34,h},dx={r.left,r.left+b,r.right-b,r.right},dy={r.top,r.top+b,r.bottom-b,r.bottom};
            for(int y=0;y<3;y++)for(int x=0;x<3;x++){if(x==1&&y==1)continue;canvas.drawBitmap(borderBitmap,new android.graphics.Rect(sx[x],sy[y],sx[x+1],sy[y+1]),new android.graphics.Rect(dx[x],dy[y],dx[x+1],dy[y+1]),paint);}
        }
        @Override public void setAlpha(int alpha){paint.setAlpha(alpha);}
        @Override public void setColorFilter(android.graphics.ColorFilter filter){paint.setColorFilter(filter);}
        @Override public int getOpacity(){return android.graphics.PixelFormat.TRANSLUCENT;}
    }
    @Override public void onBackPressed(){if(ready)engineTasks.execute(()->Mobile.back());}
    @Override public void onResume(){super.onResume();resumed=true;if(ready)engineTasks.execute(()->{Mobile.resume();handler.post(()->{if(!destroyed&&resumed)gameView.resumeGame();});});}
    @Override public void onPause(){resumed=false;if(ready)engineTasks.execute(()->{Mobile.pause();handler.post(()->{if(!destroyed&&!resumed)gameView.suspendGame();});});super.onPause();}
    @Override public void onDestroy(){destroyed=true;handler.removeCallbacksAndMessages(null);if(editorDialog!=null)editorDialog.dismiss();engineTasks.shutdown();super.onDestroy();}

    private static void removeTree(File file) throws IOException {
        if(file.isDirectory()){File[] children=file.listFiles();if(children!=null)for(File child:children)removeTree(child);}
        if(file.exists()&&!file.delete())throw new IOException("Cannot remove private staging file");
    }
    private File prepareRuntime() throws Exception {
        String expected;
        try(InputStream stream=getAssets().open("runtime.sha256")){expected=new String(readAll(stream),StandardCharsets.US_ASCII).trim();}
        File runtime=new File(getFilesDir(),"runtime"), marker=new File(runtime,".content-sha256");
        if(marker.isFile()){try(InputStream stream=new FileInputStream(marker)){if(new String(readAll(stream),StandardCharsets.US_ASCII).trim().equals(expected))return runtime;}}
        MessageDigest digest=MessageDigest.getInstance("SHA-256");
        try(InputStream stream=getAssets().open("runtime.zip")){byte[] buf=new byte[65536];int n;while((n=stream.read(buf))!=-1)digest.update(buf,0,n);}
        StringBuilder actual=new StringBuilder();for(byte b:digest.digest())actual.append(String.format("%02x",b&255));
        if(!actual.toString().equals(expected))throw new IOException("Runtime asset hash mismatch");
        File stage=new File(getFilesDir(),"runtime-incoming"),backup=new File(getFilesDir(),"runtime-previous");removeTree(stage);stage.mkdirs();
        try(ZipInputStream zip=new ZipInputStream(getAssets().open("runtime.zip"))){ZipEntry entry;byte[] buf=new byte[65536];String prefix=stage.getCanonicalPath()+File.separator;
            while((entry=zip.getNextEntry())!=null){File target=new File(stage,entry.getName());if(!target.getCanonicalPath().startsWith(prefix))throw new IOException("Unsafe runtime path");if(entry.isDirectory()){target.mkdirs();continue;}target.getParentFile().mkdirs();try(OutputStream stream=new FileOutputStream(target)){int n;while((n=zip.read(buf))!=-1)stream.write(buf,0,n);}}
        }
        try(OutputStream stream=new FileOutputStream(new File(stage,".content-sha256"))){stream.write(expected.getBytes(StandardCharsets.US_ASCII));}
        removeTree(backup);
        if(runtime.exists()&&!runtime.renameTo(backup))throw new IOException("Cannot preserve previous runtime");
        if(!stage.renameTo(runtime)){if(backup.exists())backup.renameTo(runtime);throw new IOException("Cannot install runtime");}
        removeTree(backup);return runtime;
    }
    private static byte[] readAll(InputStream stream)throws IOException{ByteArrayOutputStream out=new ByteArrayOutputStream();byte[] buf=new byte[8192];int n;while((n=stream.read(buf))!=-1)out.write(buf,0,n);return out.toByteArray();}
}
