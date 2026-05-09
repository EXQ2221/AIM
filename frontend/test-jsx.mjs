import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import fs from 'fs';
import esbuild from 'esbuild';

// Read App.tsx
const src = fs.readFileSync('src/App.tsx', 'utf8');
const lines = src.split('\n');

console.log('Total lines in App.tsx:', lines.length);

// Test 1: First half (lines 1-1060, before AuthView)
const firstPart = lines.slice(0, 1060).join('\n') + '\nexport default function Test() { return <div>ok</div>; }\n';
try {
    esbuild.transformSync(firstPart, { loader: 'tsx', jsx: 'automatic', format: 'esm' });
    console.log('Test FIRST HALF (1-1060): PASS ✓');
} catch(e) {
    console.log('Test FIRST HALF (1-1060): FAIL ✗');
    console.log(e.message);
}

// Test 2: Second half (lines 1060-2195, AuthView and after)  
const secondPart = lines.slice(1060).join('\n');
const secondWithImports = "import { useState } from 'react';\n" + secondPart + "\nexport default function Test() { return <div>ok</div>; }\n";
try {
    esbuild.transformSync(secondWithImports, { loader: 'tsx', jsx: 'automatic', format: 'esm' });
    console.log('Test SECOND HALF (1060-end): PASS ✓');
} catch(e) {
    console.log('Test SECOND HALF (1060-end): FAIL ✗');
    console.log(e.message);
    if (e.errors) e.errors.forEach(err => console.log('  ', err.text, 'at line', err.location?.line));
}
