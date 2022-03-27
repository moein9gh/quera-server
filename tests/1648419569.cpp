#include <iostream>

using namespace std;

int main()
{
    int x;
    cout<<"please enter a number bellow 12: ";
    cin>>x;

    if(x>12)
        return 2;
        
    
    
        if(x<=3)
        cout<<"first quarter";
        
       else if(x<=6)
        cout<<"second quarter";
        
       else if(x<=9)
        cout<<"third quarter";
        
       else if(x<=12)
        cout<<"fourth quarter";


    return 0;
}
