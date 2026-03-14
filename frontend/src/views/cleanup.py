import sys

path = r'e:\PERSONAL\DEVELOPMENT\REPO\filebrowserquantum\frontend\src\views\Heatmap.vue'
with open(path, 'r', encoding='utf-8') as f:
    lines = f.readlines()

# keep lines 1 to 671 (indices 0 to 670)
# skip lines 672 to 770 (indices 671 to 769)
# keep lines 771 to end (indices 770 to end)
new_lines = lines[:671] + lines[770:]

with open(path, 'w', encoding='utf-8') as f:
    f.writelines(new_lines)
print('Cleanup successful')
