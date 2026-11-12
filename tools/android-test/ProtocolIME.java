package com.anr2.mm2testime;

import android.inputmethodservice.InputMethodService;
import android.view.View;
import android.view.KeyEvent;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.widget.Button;
import android.widget.LinearLayout;

// Deterministic native IME protocol fixture. This separate test APK is never
// included in the game. It exercises a real InputConnection and composing span.
public final class ProtocolIME extends InputMethodService {
    @Override public boolean onEvaluateFullscreenMode(){return false;}
    @Override public View onCreateInputView(){
        LinearLayout root=new LinearLayout(this);root.setOrientation(LinearLayout.HORIZONTAL);root.setBackgroundColor(0xff223344);
        root.setMinimumHeight(Math.round(110*getResources().getDisplayMetrics().density));
        for(String label:new String[]{"Compose","Delete","Commit","Enter"}){
            Button button=new Button(this);button.setText(label);root.addView(button,new LinearLayout.LayoutParams(0,-1,1));
            button.setOnClickListener(v->{InputConnection input=getCurrentInputConnection();if(input==null)return;
                switch(label){case "Compose":input.setComposingText("测试long",1);break;case "Delete":input.sendKeyEvent(new KeyEvent(KeyEvent.ACTION_DOWN,KeyEvent.KEYCODE_DEL));input.sendKeyEvent(new KeyEvent(KeyEvent.ACTION_UP,KeyEvent.KEYCODE_DEL));break;case "Commit":input.commitText("测试long",1);break;case "Enter":input.performEditorAction(EditorInfo.IME_ACTION_DONE);break;}
            });
        }
        return root;
    }
}
