for ((i=0;i<50;++i))
do
   vf -v rddlock.yaml | grep Success | wc -l
done
