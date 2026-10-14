import {copyFileSync,mkdirSync} from 'node:fs';
mkdirSync('public/swagger',{recursive:true});
for(const file of ['swagger-ui.css','swagger-ui-bundle.js','LICENSE'])copyFileSync('node_modules/swagger-ui-dist/'+file,'public/swagger/'+file);
