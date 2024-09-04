DIR="dist/pinzoom"

echo "=== PinZoom Windows Dist ==="
cd $(dirname $0)
mkdir -p ${DIR} \
    ${DIR}/bin \
    ${DIR}/config \

cp server/build/bin/* ${DIR}/bin/
cp server/config/* ${DIR}/config/
rm ${DIR}/config/*.go

cp -R server/migrations ${DIR}
rm ${DIR}/migrations/*go

mkdir -p ${DIR}/static
cp -R web/build/* ${DIR}/static

# Clean up windows remote Zone.Identifier files.
find ${DIR}/bin/ -name '*:Zone.Identifier*' -delete

echo "=== Archiving ==="
cd dist
zip -r pinzoom-windows.zip pinzoom-windows
cd ../
echo "Created pinzoom-windows-x64.zip"
