const auth = new AuthBase('/auth/'); 

async function Init() {    
    try {
        // 情報を取得
        const userData = await auth.GetInfo();

        if (userData == null) {
            // ログインにリダイレクト
            window.location.href = './login.html';
            return;
        }

        console.log(userData);

        // DOMContentLoaded イベントが発生したらユーザー情報を表示
        // HTML要素を取得し、データを挿入
        document.getElementById('user-id').textContent = userData.user_id;
        document.getElementById('user-name').textContent = userData.name;
        document.getElementById('user-email').textContent = userData.email;
        document.getElementById('prov-code').textContent = userData.prov_code;
        document.getElementById('prov-uid').textContent = userData.prov_uid;

        try {
            document.getElementById('user-icon').src = auth.GetIcon(userData.user_id);
        } catch (error) {
            console.error(error);
        }

        // ログアウトボタン取得
        const logoutButton = document.getElementById('logout-button');

        // ログアウトボタンをクリックしたらログアウト
        logoutButton.addEventListener('click', async () => {
            try {
                await auth.Logout();
                // ログインにリダイレクト
                window.location.href = './login.html';
            } catch (error) {
                console.error(error);
                // ログインにリダイレクト
                window.location.href = './login.html';
            }
        });

        // アップロードボタン取得
        const uploadButton = document.getElementById('upload-button');
        // ファイル選択を取得
        const fileInput = document.getElementById('file-input');

        // アップロードボタンをクリックするとアップロード
        uploadButton.addEventListener('click',async () => {
            await auth.UpdateIcon(fileInput.files[0]);
        });

        try {
            // アクセスしてみる
            const res = await auth.Get('/app/authed',{
                'Content-Type': 'application/json',
            });

            console.log(res);
        } catch (error) {
            console.error(error);
        }

        // 本来の画面にリダイレクト
        window.location.href = '/ui/';
    } catch (error) {
        console.error(error);
        // ログインにリダイレクト
        window.location.href = './login.html';
    }
}

Init();
